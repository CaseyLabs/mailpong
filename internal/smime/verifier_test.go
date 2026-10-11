package smime

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/caseylabs/mailpong/internal/database"
	"github.com/rs/zerolog"
	"go.mozilla.org/pkcs7"
)

type testCert struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

var testSerial int64

// newTestCert creates a certificate signed by parent, or self-signed when
// parent is nil. CA certificates get no email address.
func newTestCert(t *testing.T, cn, email string, isCA bool, parent *testCert) *testCert {
	t.Helper()
	return newTestCertValid(t, cn, email, isCA, parent, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
}

// newTestCertValid is newTestCert with an explicit validity period.
func newTestCertValid(t *testing.T, cn, email string, isCA bool, parent *testCert, notBefore, notAfter time.Time) *testCert {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	testSerial++
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(testSerial),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}
	if isCA {
		tmpl.KeyUsage = x509.KeyUsageCertSign
	} else {
		tmpl.KeyUsage = x509.KeyUsageDigitalSignature
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageEmailProtection}
		tmpl.EmailAddresses = []string{email}
	}
	issuer, issuerKey := tmpl, key
	if parent != nil {
		issuer, issuerKey = parent.cert, parent.key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, issuer, &key.PublicKey, issuerKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return &testCert{cert: cert, key: key}
}

// signTestMessage returns a parsed PKCS#7 object signed by signer. Extra
// certificates are embedded before the signer's certificate.
func signTestMessage(t *testing.T, signer *testCert, extra ...*x509.Certificate) *pkcs7.PKCS7 {
	t.Helper()
	return signTestMessageWith(t, signer, pkcs7.SignerInfoConfig{}, extra...)
}

// signTestMessageWith is signTestMessage with a custom signer config.
func signTestMessageWith(t *testing.T, signer *testCert, config pkcs7.SignerInfoConfig, extra ...*x509.Certificate) *pkcs7.PKCS7 {
	t.Helper()
	sd, err := pkcs7.NewSignedData([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	sd.SetDigestAlgorithm(pkcs7.OIDDigestAlgorithmSHA256)
	for _, c := range extra {
		sd.AddCertificate(c)
	}
	if err := sd.AddSigner(signer.cert, signer.key, config); err != nil {
		t.Fatal(err)
	}
	der, err := sd.Finish()
	if err != nil {
		t.Fatal(err)
	}
	p7, err := pkcs7.Parse(der)
	if err != nil {
		t.Fatal(err)
	}
	return p7
}

func newTestVerifier(t *testing.T, roots ...*x509.Certificate) (*Verifier, *Store) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	store := NewStore(db.DB, zerolog.Nop())
	v := NewVerifier(store, zerolog.Nop())
	v.roots = x509.NewCertPool()
	for _, r := range roots {
		v.roots.AddCert(r)
	}
	return v, store
}

func TestVerifyPKCS7TrustStatus(t *testing.T) {
	trustedCA := newTestCert(t, "Trusted CA", "", true, nil)
	untrustedCA := newTestCert(t, "Attacker CA", "", true, nil)

	tests := []struct {
		name   string
		signer *testCert
		extra  []*x509.Certificate
		want   SignatureStatus
	}{
		{"trusted chain", newTestCert(t, "Alice", "alice@example.com", false, trustedCA), nil, StatusSigned},
		{"trusted chain with intermediate embedded", newTestCert(t, "Alice", "alice@example.com", false, trustedCA), []*x509.Certificate{trustedCA.cert}, StatusSigned},
		{"untrusted CA", newTestCert(t, "Mallory", "alice@example.com", false, untrustedCA), []*x509.Certificate{untrustedCA.cert}, StatusUnknownSigner},
		{"self-signed", newTestCert(t, "Alice", "alice@example.com", false, nil), nil, StatusSelfSigned},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, _ := newTestVerifier(t, trustedCA.cert)
			result := v.verifyPKCS7(signTestMessage(t, tt.signer, tt.extra...))
			if result.Status != tt.want {
				t.Fatalf("status = %q (%s), want %q", result.Status, result.ErrorMessage, tt.want)
			}
		})
	}
}

func TestVerifyPKCS7UsesActualSignerCert(t *testing.T) {
	ca := newTestCert(t, "Trusted CA", "", true, nil)
	signer := newTestCert(t, "Mallory", "mallory@example.com", false, ca)
	decoy := newTestCert(t, "Alice", "alice@example.com", false, ca)

	v, store := newTestVerifier(t, ca.cert)
	result := v.verifyPKCS7(signTestMessage(t, signer, decoy.cert))

	if result.Status != StatusSigned {
		t.Fatalf("status = %q (%s), want %q", result.Status, result.ErrorMessage, StatusSigned)
	}
	if result.SignerEmail != "mallory@example.com" || result.SignerName != "Mallory" {
		t.Errorf("signer = %q <%s>, want Mallory <mallory@example.com>", result.SignerName, result.SignerEmail)
	}

	if certs, err := store.GetSenderCerts("alice@example.com"); err != nil || len(certs) != 0 {
		t.Errorf("decoy cached for alice: %d certs, err %v", len(certs), err)
	}
	certs, err := store.GetSenderCerts("mallory@example.com")
	if err != nil || len(certs) != 1 {
		t.Fatalf("mallory certs = %d, err %v; want 1", len(certs), err)
	}
	if certs[0].SerialNumber != signer.cert.SerialNumber.String() {
		t.Errorf("cached serial %s, want signer serial %s", certs[0].SerialNumber, signer.cert.SerialNumber)
	}
}

func TestVerifyPKCS7ChecksEverySigner(t *testing.T) {
	trustedCA := newTestCert(t, "Trusted CA", "", true, nil)
	untrustedCA := newTestCert(t, "Attacker CA", "", true, nil)
	alice := newTestCert(t, "Alice", "alice@example.com", false, trustedCA)
	mallory := newTestCert(t, "Mallory", "mallory@example.com", false, untrustedCA)

	sd, err := pkcs7.NewSignedData([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	sd.SetDigestAlgorithm(pkcs7.OIDDigestAlgorithmSHA256)
	for _, s := range []*testCert{alice, mallory} {
		if err := sd.AddSigner(s.cert, s.key, pkcs7.SignerInfoConfig{}); err != nil {
			t.Fatal(err)
		}
	}
	der, err := sd.Finish()
	if err != nil {
		t.Fatal(err)
	}
	p7, err := pkcs7.Parse(der)
	if err != nil {
		t.Fatal(err)
	}

	v, _ := newTestVerifier(t, trustedCA.cert)
	if result := v.verifyPKCS7(p7); result.Status != StatusUnknownSigner {
		t.Fatalf("status = %q (%s), want %q", result.Status, result.ErrorMessage, StatusUnknownSigner)
	}
}

func TestUntrustedSignerCertNotUsedForEncryption(t *testing.T) {
	ca := newTestCert(t, "Trusted CA", "", true, nil)
	attackerCA := newTestCert(t, "Attacker CA", "", true, nil)
	alice := newTestCert(t, "Alice", "alice@example.com", false, ca)

	v, store := newTestVerifier(t, ca.cert)
	if r := v.verifyPKCS7(signTestMessage(t, alice)); r.Status != StatusSigned {
		t.Fatalf("status = %q, want signed", r.Status)
	}

	alicePEM := func() string {
		t.Helper()
		pems, err := store.GetSenderCertPEMs([]string{"alice@example.com"})
		if err != nil {
			t.Fatal(err)
		}
		return pems["alice@example.com"]
	}
	trustedPEM := alicePEM()
	if trustedPEM == "" {
		t.Fatal("chain-trusted cert should be usable for encryption")
	}

	// Later forged signatures claiming alice must not replace her key.
	for _, forger := range []*testCert{
		newTestCert(t, "Mallory", "alice@example.com", false, attackerCA),
		newTestCert(t, "Mallory", "alice@example.com", false, nil),
	} {
		time.Sleep(time.Millisecond) // newer last_seen_at
		v.verifyPKCS7(signTestMessage(t, forger))
		if got := alicePEM(); got != trustedPEM {
			t.Fatalf("untrusted signer cert became alice's encryption key")
		}
	}

	// An untrusted cert alone is not used, but explicit import accepts it.
	bob := newTestCert(t, "Bob", "bob@example.com", false, nil)
	v.verifyPKCS7(signTestMessage(t, bob))
	if pems, _ := store.GetSenderCertPEMs([]string{"bob@example.com"}); pems["bob@example.com"] != "" {
		t.Fatal("self-signed cert should not be used without explicit import")
	}
	if err := store.ImportSenderCertFromFile("bob@example.com", bob.cert.Raw); err != nil {
		t.Fatal(err)
	}
	if pems, _ := store.GetSenderCertPEMs([]string{"bob@example.com"}); pems["bob@example.com"] == "" {
		t.Fatal("imported cert should be usable for encryption")
	}
}

func TestVerifyPKCS7ChecksChainAtCurrentTime(t *testing.T) {
	ca := newTestCertValid(t, "Trusted CA", "", true, nil, time.Now().AddDate(-1, 0, 0), time.Now().AddDate(1, 0, 0))
	expired := newTestCertValid(t, "Alice", "alice@example.com", false, ca, time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))
	backdated := time.Now().Add(-36 * time.Hour).UTC()

	p7 := signTestMessageWith(t, expired, pkcs7.SignerInfoConfig{
		ExtraSignedAttributes: []pkcs7.Attribute{{Type: pkcs7.OIDAttributeSigningTime, Value: backdated}},
	})
	var claimed time.Time
	if err := p7.UnmarshalSignedAttribute(pkcs7.OIDAttributeSigningTime, &claimed); err != nil || !claimed.Equal(backdated.Truncate(time.Second)) {
		t.Fatalf("test message claims signing time %v (%v), want the backdated %v", claimed, err, backdated)
	}

	v, _ := newTestVerifier(t, ca.cert)
	if result := v.verifyPKCS7(p7); result.Status != StatusExpiredCert {
		t.Fatalf("status = %q (%s), want %q for a signature claiming a time inside an expired cert's validity", result.Status, result.ErrorMessage, StatusExpiredCert)
	}
}

func TestCheckSender(t *testing.T) {
	ca := newTestCert(t, "Trusted CA", "", true, nil)
	signer := newTestCert(t, "Alice", "Alice@Example.com", false, ca)

	tests := []struct {
		name   string
		status SignatureStatus
		from   string
		want   SignatureStatus
	}{
		{"matching from", StatusSigned, "alice@example.com", StatusSigned},
		{"forged from", StatusSigned, "ceo@example.com", StatusSignerMismatch},
		{"empty from", StatusSigned, "", StatusSignerMismatch},
		{"untrusted left alone", StatusUnknownSigner, "ceo@example.com", StatusUnknownSigner},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, _ := newTestVerifier(t, ca.cert)
			result := v.verifyPKCS7(signTestMessage(t, signer))
			result.Status = tt.status
			result.CheckSender(tt.from)
			if result.Status != tt.want {
				t.Fatalf("status = %q, want %q", result.Status, tt.want)
			}
		})
	}
}

func TestCertEmailsIncludesSubjectEmailAddress(t *testing.T) {
	cert := &x509.Certificate{
		EmailAddresses: []string{"alt@example.com"},
		Subject: pkix.Name{Names: []pkix.AttributeTypeAndValue{
			{Type: oidEmailAddress, Value: "subject@example.com"},
		}},
	}
	got := certEmails(cert)
	if len(got) != 2 || got[0] != "alt@example.com" || got[1] != "subject@example.com" {
		t.Fatalf("certEmails = %v", got)
	}
}
