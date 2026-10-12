package smime

import (
	"bytes"
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sync"
	"time"

	"golang.org/x/crypto/ocsp"
)

var (
	errRevocationUnchecked = errors.New("certificate revocation was not checked; online checks are disabled")
	errRevocationUnknown   = errors.New("certificate revocation status is unavailable")
	errCertificateRevoked  = errors.New("certificate has been revoked")
)

type revocationCache struct {
	err     error
	expires time.Time
}
type revocationChecker struct {
	client *http.Client
	mu     sync.Mutex
	cache  map[[32]byte]revocationCache
}

func newRevocationChecker() *revocationChecker {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Endpoints come from certificates. Dial the validated public IP directly
	// so a certificate cannot probe private services or rebind DNS between checks.
	transport.Proxy = nil
	transport.DialContext = publicRevocationDial
	return &revocationChecker{client: &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("too many revocation redirects")
		}
		return validRevocationURL(req.URL)
	}}, cache: make(map[[32]byte]revocationCache)}
}

func validRevocationURL(u *url.URL) error {
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return errors.New("unsupported revocation endpoint")
	}
	return nil
}
func publicRevocationDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if !publicRevocationIP(ip.IP) {
			return nil, errors.New("revocation endpoint is not public")
		}
	}
	var last error
	for _, ip := range ips {
		conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		last = err
	}
	if last == nil {
		last = errors.New("revocation endpoint has no address")
	}
	return nil, last
}

func (c *revocationChecker) checkChain(chain []*x509.Certificate) error {
	return c.checkChains([][]*x509.Certificate{chain})
}

// One current good path is sufficient. A failed path does not revoke an
// alternative valid chain, and every alternative shares one request budget.
func (c *revocationChecker) checkChains(chains [][]*x509.Certificate) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var unavailable, revoked error
	for _, chain := range chains {
		var chainFailure error
		if len(chain) < 2 {
			unavailable = errRevocationUnknown
			continue
		}
		for i := 0; i < len(chain)-1; i++ {
			err := c.checkCertificate(ctx, chain[i], chain[i+1])
			if errors.Is(err, errCertificateRevoked) {
				chainFailure = err
				break
			}
			if err != nil {
				chainFailure = err
			}
		}
		if chainFailure == nil {
			return nil
		}
		if errors.Is(chainFailure, errCertificateRevoked) {
			revoked = chainFailure
		} else {
			unavailable = chainFailure
		}
	}
	if unavailable != nil {
		return unavailable
	}
	if revoked != nil {
		return revoked
	}
	return errRevocationUnknown
}

func (c *revocationChecker) checkCertificate(ctx context.Context, cert, issuer *x509.Certificate) error {
	key := revocationCacheKey(cert, issuer)
	now := time.Now()
	c.mu.Lock()
	cached, ok := c.cache[key]
	c.mu.Unlock()
	if ok && now.Before(cached.expires) {
		return cached.err
	}
	var last error
	for i, endpoint := range cert.OCSPServer {
		if i >= 2 {
			break
		}
		err, expires := c.checkOCSP(ctx, endpoint, cert, issuer, now)
		if err == nil || errors.Is(err, errCertificateRevoked) {
			c.remember(key, err, expires, now)
			return err
		}
		last = err
	}
	for i, endpoint := range cert.CRLDistributionPoints {
		if i >= 2 {
			break
		}
		err, expires := c.checkCRL(ctx, endpoint, cert, issuer, now)
		if err == nil || errors.Is(err, errCertificateRevoked) {
			c.remember(key, err, expires, now)
			return err
		}
		last = err
	}
	if last == nil {
		return errRevocationUnknown
	}
	return fmt.Errorf("%w: %v", errRevocationUnknown, last)
}
func (c *revocationChecker) remember(key [32]byte, err error, expires, now time.Time) {
	if expires.After(now.Add(time.Hour)) {
		expires = now.Add(time.Hour)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cache) >= 256 {
		clear(c.cache)
	}
	c.cache[key] = revocationCache{err: err, expires: expires}
}
func (c *revocationChecker) fetch(ctx context.Context, endpoint, method string, body []byte) ([]byte, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	if err = validRevocationURL(u); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/ocsp-request")
		req.Header.Set("Accept", "application/ocsp-response")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("revocation endpoint returned HTTP %d", resp.StatusCode)
	}
	const limit = 4 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, errors.New("revocation response too large")
	}
	return data, nil
}
func (c *revocationChecker) checkOCSP(ctx context.Context, endpoint string, cert, issuer *x509.Certificate, now time.Time) (error, time.Time) {
	request, err := ocsp.CreateRequest(cert, issuer, nil)
	if err != nil {
		return err, time.Time{}
	}
	data, err := c.fetch(ctx, endpoint, http.MethodPost, request)
	if err != nil {
		return err, time.Time{}
	}
	response, err := ocsp.ParseResponseForCert(data, cert, issuer)
	if err != nil {
		return err, time.Time{}
	}
	if err = validateOCSPIssuer(data, cert, issuer, response.IssuerHash); err != nil {
		return err, time.Time{}
	}
	expires := response.NextUpdate
	if expires.IsZero() {
		expires = response.ThisUpdate.Add(24 * time.Hour)
	}
	if response.ThisUpdate.IsZero() || response.ThisUpdate.After(now.Add(5*time.Minute)) || !now.Before(expires) || !expires.After(response.ThisUpdate) || response.ProducedAt.After(now.Add(5*time.Minute)) {
		return errors.New("OCSP response is not current"), time.Time{}
	}
	if delegated := response.Certificate; delegated != nil && !bytes.Equal(delegated.Raw, issuer.Raw) {
		if !bytes.Equal(delegated.RawIssuer, issuer.RawSubject) || delegated.CheckSignatureFrom(issuer) != nil || len(delegated.UnhandledCriticalExtensions) > 0 {
			return errors.New("OCSP responder does not identify the direct issuer"), time.Time{}
		}
		if now.Before(delegated.NotBefore) || now.After(delegated.NotAfter) {
			return errors.New("OCSP responder certificate is not current"), time.Time{}
		}
		allowed := false
		for _, usage := range delegated.ExtKeyUsage {
			if usage == x509.ExtKeyUsageOCSPSigning {
				allowed = true
			}
		}
		if expires.After(delegated.NotAfter) {
			expires = delegated.NotAfter
		}
		if !allowed {
			return errors.New("OCSP responder is not authorized for OCSP signing"), time.Time{}
		}
	}
	switch response.Status {
	case ocsp.Good:
		return nil, expires
	case ocsp.Revoked:
		return errCertificateRevoked, expires
	default:
		return errRevocationUnknown, time.Time{}
	}
}
func (c *revocationChecker) checkCRL(ctx context.Context, endpoint string, cert, issuer *x509.Certificate, now time.Time) (error, time.Time) {
	data, err := c.fetch(ctx, endpoint, http.MethodGet, nil)
	if err != nil {
		return err, time.Time{}
	}
	list, err := x509.ParseRevocationList(data)
	if err != nil {
		return err, time.Time{}
	}
	if !bytes.Equal(list.RawIssuer, issuer.RawSubject) {
		return errors.New("CRL issuer does not match certificate issuer"), time.Time{}
	}
	if err = list.CheckSignatureFrom(issuer); err != nil {
		return err, time.Time{}
	}
	if list.ThisUpdate.IsZero() || list.ThisUpdate.After(now.Add(5*time.Minute)) || list.NextUpdate.IsZero() || !now.Before(list.NextUpdate) {
		return errors.New("CRL is not current"), time.Time{}
	}
	for _, ext := range list.Extensions {
		// Partial/indirect/delta lists need additional scope and base-list handling.
		if ext.Id.Equal(asn1.ObjectIdentifier{2, 5, 29, 27}) || ext.Id.Equal(asn1.ObjectIdentifier{2, 5, 29, 28}) || ext.Critical {
			return errors.New("unsupported CRL scope or critical extension"), time.Time{}
		}
	}
	for _, entry := range list.RevokedCertificateEntries {
		if cert.SerialNumber.Cmp(entry.SerialNumber) == 0 {
			return errCertificateRevoked, list.NextUpdate
		}
	}
	return nil, list.NextUpdate
}

var nonPublicRevocationRanges = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2001::/32"), netip.MustParsePrefix("2002::/16"),
}

func publicRevocationIP(ip net.IP) bool {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	address = address.Unmap()
	if address.Is6() && !netip.MustParsePrefix("2000::/3").Contains(address) {
		return false
	}
	for _, blocked := range nonPublicRevocationRanges {
		if blocked.Contains(address) {
			return false
		}
	}
	return true
}

// ParseResponseForCert authenticates the responder and serial, but the pinned
// library does not bind the response CertID's issuer hashes to our issuer.
// Extract only that identity from the already signature-validated DER.
func validateOCSPIssuer(data []byte, cert, issuer *x509.Certificate, hash crypto.Hash) error {
	var outer struct {
		Status   asn1.Enumerated
		Response struct {
			Type  asn1.ObjectIdentifier
			Bytes []byte
		} `asn1:"explicit,tag:0,optional"`
	}
	if _, err := asn1.Unmarshal(data, &outer); err != nil {
		return err
	}
	var basic struct {
		Data struct {
			Version    int `asn1:"optional,default:0,explicit,tag:0"`
			Responder  asn1.RawValue
			ProducedAt time.Time `asn1:"generalized"`
			Responses  []asn1.RawValue
		}
		Algorithm    pkix.AlgorithmIdentifier
		Signature    asn1.BitString
		Certificates []asn1.RawValue `asn1:"explicit,tag:0,optional"`
	}
	if _, err := asn1.Unmarshal(outer.Response.Bytes, &basic); err != nil {
		return err
	}
	request, err := ocsp.CreateRequest(cert, issuer, &ocsp.RequestOptions{Hash: hash})
	if err != nil {
		return err
	}
	expected, err := ocsp.ParseRequest(request)
	if err != nil {
		return err
	}
	for _, single := range basic.Data.Responses {
		var id struct {
			Algorithm         pkix.AlgorithmIdentifier
			NameHash, KeyHash []byte
			Serial            *big.Int
		}
		if _, err := asn1.Unmarshal(single.Bytes, &id); err != nil {
			return err
		}
		if id.Serial != nil && id.Serial.Cmp(cert.SerialNumber) == 0 {
			if !bytes.Equal(id.NameHash, expected.IssuerNameHash) || !bytes.Equal(id.KeyHash, expected.IssuerKeyHash) {
				return errors.New("OCSP certificate issuer identity does not match")
			}
			return nil
		}
	}
	return errors.New("OCSP response does not identify the certificate")
}

func revocationCacheKey(cert, issuer *x509.Certificate) [32]byte {
	return sha256.Sum256(append(append([]byte{}, cert.Raw...), issuer.Raw...))
}
