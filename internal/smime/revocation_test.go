package smime

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.mozilla.org/pkcs7"
	"golang.org/x/crypto/ocsp"
)

type revocationRoundTrip func(*http.Request) (*http.Response, error)

func (f revocationRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func revocationSigner(t *testing.T, ca *testCert, crl bool) *testCert {
	signer := newTestCert(t, "Alice", "alice@example.com", false, ca)
	template := *signer.cert
	if crl {
		template.CRLDistributionPoints = []string{"https://status.example.org/crl"}
	} else {
		template.OCSPServer = []string{"https://status.example.org/ocsp"}
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, ca.cert, &signer.key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}
	signer.cert, err = x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func TestSMIMEOnlineRevocation(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		crl                             bool
		status                          int
		stale, wrongSigner, wrongHashes bool
		want                            SignatureStatus
	}{
		{name: "OCSP good", status: ocsp.Good, want: StatusSigned},
		{name: "OCSP revoked", status: ocsp.Revoked, want: StatusRevokedCert},
		{name: "OCSP unknown", status: ocsp.Unknown, want: StatusRevocationUnknown},
		{name: "OCSP stale", status: ocsp.Good, stale: true, want: StatusRevocationUnknown},
		{name: "OCSP wrong issuer hashes", status: ocsp.Good, wrongHashes: true, want: StatusRevocationUnknown},
		{name: "OCSP wrong issuer", status: ocsp.Good, wrongSigner: true, want: StatusRevocationUnknown},
		{name: "CRL good", crl: true, want: StatusSigned},
		{name: "CRL revoked", crl: true, status: ocsp.Revoked, want: StatusRevokedCert},
		{name: "CRL stale", crl: true, stale: true, want: StatusRevocationUnknown},
		{name: "CRL wrong issuer", crl: true, wrongSigner: true, want: StatusRevocationUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ca := newTestCert(t, "CA", "", true, nil)
			signer := revocationSigner(t, ca, tc.crl)
			signingCA := ca
			if tc.wrongSigner {
				signingCA = newTestCert(t, "Other CA", "", true, nil)
			}
			now := time.Now()
			thisUpdate := now.Add(-time.Minute)
			nextUpdate := now.Add(time.Hour)
			if tc.stale {
				thisUpdate = now.Add(-2 * time.Hour)
				nextUpdate = now.Add(-time.Hour)
			}
			var data []byte
			var err error
			if tc.crl {
				list := &x509.RevocationList{Number: big.NewInt(1), ThisUpdate: thisUpdate, NextUpdate: nextUpdate}
				if tc.status == ocsp.Revoked {
					list.RevokedCertificateEntries = []x509.RevocationListEntry{{SerialNumber: signer.cert.SerialNumber, RevocationTime: now.Add(-time.Hour)}}
				}
				data, err = x509.CreateRevocationList(rand.Reader, list, signingCA.cert, signingCA.key)
			} else {
				issuerForID := signingCA.cert
				if tc.wrongHashes {
					issuerForID = newTestCert(t, "Different issuer identity", "", true, nil).cert
				}
				data, err = ocsp.CreateResponse(issuerForID, signingCA.cert, ocsp.Response{SerialNumber: signer.cert.SerialNumber, Status: tc.status, ThisUpdate: thisUpdate, NextUpdate: nextUpdate, RevokedAt: now.Add(-time.Hour)}, signingCA.key)
			}
			if err != nil {
				t.Fatal(err)
			}
			requests := 0
			enabled := false
			v, store := newTestVerifier(t, ca.cert)
			v.SetOnlineRevocationCheck(func() bool { return enabled })
			v.revocation.client = &http.Client{Transport: revocationRoundTrip(func(req *http.Request) (*http.Response, error) {
				requests++
				return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Header: make(http.Header)}, nil
			})}
			signed := signTestMessage(t, signer)
			if got := v.verifyPKCS7(signed); got.Status != StatusRevocationUnchecked || requests != 0 {
				t.Fatalf("offline result=%+v,requests=%d", got, requests)
			}
			keys, err := store.GetSenderCertPEMs([]string{"alice@example.com"})
			if err != nil || keys["alice@example.com"] == "" {
				t.Fatalf("offline trusted signer lost encryption eligibility: %v", err)
			}
			enabled = true
			got := v.verifyPKCS7(signed)
			if got.Status != tc.want {
				t.Fatalf("online result=%+v,want=%s", got, tc.want)
			}
			got.CheckSender("ceo@example.com")
			if tc.want == StatusSigned || tc.want == StatusRevocationUnknown {
				if got.Status != StatusSignerMismatch {
					t.Fatal("forged From was not rejected")
				}
			}
			if tc.want == StatusSigned || tc.want == StatusRevokedCert {
				before := requests
				v.verifyPKCS7(signed)
				if requests != before {
					t.Fatal("current revocation result not cached")
				}
			}
			keys, err = store.GetSenderCertPEMs([]string{"alice@example.com"})
			if err != nil {
				t.Fatal(err)
			}
			if (keys["alice@example.com"] == "") != (tc.want == StatusRevokedCert) {
				t.Fatalf("encryption eligibility for %s: %v", tc.want, keys)
			}
			enabled = false
			before := requests
			offlineWant := StatusRevocationUnchecked
			if tc.want == StatusRevokedCert {
				offlineWant = StatusRevokedCert
			}
			if got := v.verifyPKCS7(signed); got.Status != offlineWant {
				t.Fatalf("opt-out ignored: %+v", got)
			}
			if requests != before {
				t.Fatal("opt-out made a network request")
			}
		})
	}
}

func TestRevocationUnavailableAndEndpoints(t *testing.T) {
	ca := newTestCert(t, "CA", "", true, nil)
	signer := newTestCert(t, "Alice", "alice@example.com", false, ca)
	checker := newRevocationChecker()
	if err := checker.checkChain([]*x509.Certificate{signer.cert, ca.cert}); !errors.Is(err, errRevocationUnknown) {
		t.Fatalf("no endpoints error=%v", err)
	}
	for _, raw := range []string{"file:///etc/passwd", "http://user:pass@example.org/ocsp", "data:text/plain,abc"} {
		parsed, _ := url.Parse(raw)
		if validRevocationURL(parsed) == nil {
			t.Fatalf("accepted endpoint %q", raw)
		}
	}
	for _, address := range []string{"127.0.0.1:80", "[::1]:80", "169.254.169.254:80", "10.0.0.1:80", "100.64.0.1:80", "198.18.0.1:80"} {
		conn, err := publicRevocationDial(context.Background(), "tcp", address)
		if conn != nil {
			conn.Close()
		}
		if err == nil {
			t.Fatalf("private endpoint accepted: %s", address)
		}
	}
	// A failed HTTP response must not turn into a trusted signature.
	checker.client = &http.Client{Transport: revocationRoundTrip(func(*http.Request) (*http.Response, error) {
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("offline")}
	})}
	signer = revocationSigner(t, ca, false)
	if err := checker.checkChain([]*x509.Certificate{signer.cert, ca.cert}); !errors.Is(err, errRevocationUnknown) {
		t.Fatalf("offline error=%v", err)
	}
}

func TestCRLRejectsPartialList(t *testing.T) {
	ca := newTestCert(t, "CA", "", true, nil)
	signer := revocationSigner(t, ca, true)
	now := time.Now()
	list := &x509.RevocationList{Number: big.NewInt(1), ThisUpdate: now.Add(-time.Minute), NextUpdate: now.Add(time.Hour), ExtraExtensions: []pkix.Extension{{Id: []int{2, 5, 29, 27}, Value: []byte{2, 1, 1}}}}
	data, err := x509.CreateRevocationList(rand.Reader, list, ca.cert, ca.key)
	if err != nil {
		t.Fatal(err)
	}
	checker := newRevocationChecker()
	checker.client = &http.Client{Transport: revocationRoundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(data))}, nil
	})}
	err = checker.checkChain([]*x509.Certificate{signer.cert, ca.cert})
	if !errors.Is(err, errRevocationUnknown) || !strings.Contains(err.Error(), "unsupported CRL") {
		t.Fatalf("partial CRL error=%v", err)
	}
}

func TestRevocationTransportIgnoresProxy(t *testing.T) {
	requests := 0
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.WriteHeader(200) }))
	defer proxy.Close()
	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("HTTPS_PROXY", proxy.URL)
	checker := newRevocationChecker()
	transport := checker.client.Transport.(*http.Transport)
	if transport.Proxy != nil {
		t.Fatal("certificate requests may be delegated to an environment proxy")
	}
	if _, err := checker.fetch(context.Background(), "http://127.0.0.1/private", http.MethodGet, nil); err == nil {
		t.Fatal("private destination accepted")
	}
	if requests != 0 {
		t.Fatal("proxy was contacted")
	}
	for _, raw := range []string{"8.8.8.8", "1.1.1.1", "2001:4860:4860::8888"} {
		if !publicRevocationIP(net.ParseIP(raw)) {
			t.Errorf("public IP rejected: %s", raw)
		}
	}
}

func TestRevokedIntermediateWinsOverUnavailableLeaf(t *testing.T) {
	root := newTestCert(t, "Root", "", true, nil)
	intermediate := newTestCert(t, "Intermediate", "", true, root)
	template := *intermediate.cert
	template.CRLDistributionPoints = []string{"https://status.example.org/intermediate.crl"}
	der, err := x509.CreateCertificate(rand.Reader, &template, root.cert, &intermediate.key.PublicKey, root.key)
	if err != nil {
		t.Fatal(err)
	}
	intermediate.cert, err = x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	leaf := newTestCert(t, "Alice", "alice@example.com", false, intermediate)
	now := time.Now()
	list, err := x509.CreateRevocationList(rand.Reader, &x509.RevocationList{Number: big.NewInt(1), ThisUpdate: now.Add(-time.Minute), NextUpdate: now.Add(time.Hour), RevokedCertificateEntries: []x509.RevocationListEntry{{SerialNumber: intermediate.cert.SerialNumber, RevocationTime: now.Add(-time.Hour)}}}, root.cert, root.key)
	if err != nil {
		t.Fatal(err)
	}
	checker := newRevocationChecker()
	checker.client = &http.Client{Transport: revocationRoundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(list))}, nil
	})}
	if err := checker.checkChain([]*x509.Certificate{leaf.cert, intermediate.cert, root.cert}); !errors.Is(err, errCertificateRevoked) {
		t.Fatalf("revoked intermediate masked: %v", err)
	}
}

func TestRevocationAlternativePaths(t *testing.T) {
	rootA := newTestCert(t, "Root A", "", true, nil)
	rootB := newTestCert(t, "Root B", "", true, nil)
	intermediateA := newTestCert(t, "Intermediate", "", true, rootA)
	template := *intermediateA.cert
	template.SerialNumber = big.NewInt(9999)
	der, err := x509.CreateCertificate(rand.Reader, &template, rootB.cert, &intermediateA.key.PublicKey, rootB.key)
	if err != nil {
		t.Fatal(err)
	}
	intermediateB, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	leaf := newTestCert(t, "Alice", "alice@example.com", false, intermediateA)
	chains := [][]*x509.Certificate{{leaf.cert, intermediateA.cert, rootA.cert}, {leaf.cert, intermediateB, rootB.cert}}
	for _, tc := range []struct {
		name   string
		second error
		want   error
	}{
		{"good alternative", nil, nil},
		{"all revoked", errCertificateRevoked, errCertificateRevoked},
		{"unavailable alternative", errRevocationUnknown, errRevocationUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checker := newRevocationChecker()
			now := time.Now()
			// Use the same signed-status cache as a successful online response.
			// An uncached certificate has no endpoints and remains unknown.
			for _, chain := range chains {
				checker.remember(revocationCacheKey(chain[0], chain[1]), nil, now.Add(time.Hour), now)
			}
			checker.remember(revocationCacheKey(intermediateA.cert, rootA.cert), errCertificateRevoked, now.Add(time.Hour), now)
			if tc.second != errRevocationUnknown {
				checker.remember(revocationCacheKey(intermediateB, rootB.cert), tc.second, now.Add(time.Hour), now)
			}
			if err := checker.checkChains(chains); !errors.Is(err, tc.want) {
				t.Fatalf("result=%v,want=%v", err, tc.want)
			}
		})
	}
	v, _ := newTestVerifier(t, rootA.cert, rootB.cert)
	v.revocationCheck = func(paths [][]*x509.Certificate) error {
		if len(paths) != 2 {
			t.Fatalf("verifier supplied %d paths,want 2", len(paths))
		}
		return nil
	}
	signed := signTestMessage(t, leaf, intermediateA.cert, intermediateB)
	if got := v.verifyPKCS7(signed); got.Status != StatusSigned {
		t.Fatalf("alternate paths not verified: %+v", got)
	}
}

func TestRevocationPersistsForActualSigner(t *testing.T) {
	ca := newTestCert(t, "CA", "", true, nil)
	alice := newTestCert(t, "Alice", "alice@example.com", false, ca)
	bob := newTestCert(t, "Bob", "bob@example.com", false, ca)
	v, store := newTestVerifier(t, ca.cert)
	for _, signer := range []*testCert{alice, bob} {
		v.verifyPKCS7(signTestMessage(t, signer))
	}
	sd, err := pkcs7.NewSignedData([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	sd.SetDigestAlgorithm(pkcs7.OIDDigestAlgorithmSHA256)
	for _, signer := range []*testCert{alice, bob} {
		if err := sd.AddSigner(signer.cert, signer.key, pkcs7.SignerInfoConfig{}); err != nil {
			t.Fatal(err)
		}
	}
	der, err := sd.Finish()
	if err != nil {
		t.Fatal(err)
	}
	signed, err := pkcs7.Parse(der)
	if err != nil {
		t.Fatal(err)
	}
	signers := signerCertificates(signed)
	revoked := signers[1]
	v.revocationCheck = func(chains [][]*x509.Certificate) error {
		if bytes.Equal(chains[0][0].Raw, revoked.Raw) {
			return errCertificateRevoked
		}
		return nil
	}
	if got := v.verifyPKCS7(signed); got.Status != StatusRevokedCert {
		t.Fatalf("status=%s", got.Status)
	}
	for i, cert := range signers {
		marked, err := store.SenderCertRevoked(certificateFingerprint(cert.Raw))
		if err != nil || marked != (i == 1) {
			t.Fatalf("signer %d revoked=%v err=%v", i, marked, err)
		}
	}
	keys, err := store.GetSenderCertPEMs([]string{"alice@example.com", "bob@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	revokedEmail, _ := signerIdentity(revoked)
	if len(keys) != 1 || keys[revokedEmail] != "" {
		t.Fatalf("revoked encryption candidate: %v", keys)
	}
	v.revocationCheck = func([][]*x509.Certificate) error { return errRevocationUnchecked }
	if got := v.verifyPKCS7(signed); got.Status != StatusRevokedCert {
		t.Fatalf("offline forgot later signer revocation: %s", got.Status)
	}
	// Give the first signer a prior negative marker, then verify its good path
	// while the second signer is still revoked. Trust must be independent.
	if err := store.SetSenderCertRevoked(certificateFingerprint(signers[0].Raw), true); err != nil {
		t.Fatal(err)
	}
	v.revocationCheck = func(chains [][]*x509.Certificate) error {
		if bytes.Equal(chains[0][0].Raw, revoked.Raw) {
			return errCertificateRevoked
		}
		return nil
	}
	v.verifyPKCS7(signed)
	firstEmail, _ := signerIdentity(signers[0])
	keys, err = store.GetSenderCertPEMs([]string{firstEmail, revokedEmail})
	if err != nil || keys[firstEmail] == "" || keys[revokedEmail] != "" {
		t.Fatalf("signer-specific online recovery: %v %v", keys, err)
	}
	v.revocationCheck = func([][]*x509.Certificate) error { return nil }
	if got := v.verifyPKCS7(signed); got.Status != StatusSigned {
		t.Fatalf("good online result=%s", got.Status)
	}
	marked, err := store.SenderCertRevoked(certificateFingerprint(revoked.Raw))
	if err != nil || marked {
		t.Fatalf("fresh good result retained revocation: %v %v", marked, err)
	}
	// Both current-good signers regain encryption eligibility in this message.
	keys, err = store.GetSenderCertPEMs([]string{"alice@example.com", "bob@example.com"})
	if err != nil || len(keys) != 2 {
		t.Fatalf("fresh good encryption candidates: %v %v", keys, err)
	}
}

func TestOfflineVerifierSkipsOnlineRequests(t *testing.T) {
	ca := newTestCert(t, "CA", "", true, nil)
	signer := newTestCert(t, "Alice", "alice@example.com", false, ca)
	v, _ := newTestVerifier(t, ca.cert)
	requests := 0
	v.revocationCheck = func([][]*x509.Certificate) error { requests++; return nil }
	offline := v.Offline()
	if got := offline.verifyPKCS7(signTestMessage(t, signer)); got.Status != StatusRevocationUnchecked || requests != 0 {
		t.Fatalf("offline result=%+v requests=%d", got, requests)
	}
	if got := v.verifyPKCS7(signTestMessage(t, signer)); got.Status != StatusSigned || requests != 1 {
		t.Fatalf("online verifier changed: %+v requests=%d", got, requests)
	}
}

func TestRevocationCheckedAfterUntrustedSigner(t *testing.T) {
	trustedCA := newTestCert(t, "Trusted CA", "", true, nil)
	otherCA := newTestCert(t, "Other CA", "", true, nil)
	untrusted := newTestCert(t, "Untrusted", "untrusted@example.com", false, otherCA)
	revoked := newTestCert(t, "Revoked", "revoked@example.com", false, trustedCA)
	v, store := newTestVerifier(t, trustedCA.cert)
	v.verifyPKCS7(signTestMessage(t, revoked))
	sd, err := pkcs7.NewSignedData([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	sd.SetDigestAlgorithm(pkcs7.OIDDigestAlgorithmSHA256)
	for _, signer := range []*testCert{untrusted, revoked} {
		if err := sd.AddSigner(signer.cert, signer.key, pkcs7.SignerInfoConfig{}); err != nil {
			t.Fatal(err)
		}
	}
	der, err := sd.Finish()
	if err != nil {
		t.Fatal(err)
	}
	signed, err := pkcs7.Parse(der)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(signerCertificates(signed)[0].Raw, untrusted.cert.Raw) {
		t.Fatal("fixture did not put untrusted signer first")
	}
	checks := 0
	v.revocationCheck = func([][]*x509.Certificate) error { checks++; return errCertificateRevoked }
	if got := v.verifyPKCS7(signed); got.Status != StatusRevokedCert || checks != 1 {
		t.Fatalf("later signer skipped: %+v checks=%d", got, checks)
	}
	keys, err := store.GetSenderCertPEMs([]string{"revoked@example.com"})
	if err != nil || len(keys) != 0 {
		t.Fatalf("revoked co-signer still eligible: %v %v", keys, err)
	}
}
