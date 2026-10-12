package smime

import (
	"bytes"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"go.mozilla.org/pkcs7"
)

// Verifier handles S/MIME signature verification
type Verifier struct {
	store           *Store
	log             zerolog.Logger
	roots           *x509.CertPool // trusted roots; nil uses the system pool
	revocationCheck func([][]*x509.Certificate) error
	revocation      *revocationChecker
}

// NewVerifier creates a new S/MIME verifier
func NewVerifier(store *Store, log zerolog.Logger) *Verifier {
	return &Verifier{
		store: store,
		log:   log,
	}
}

// Offline returns a verifier sharing the certificate store and trust roots,
// with online requests disabled. Sync extraction must not spend its body
// parsing deadline waiting on certificate authority endpoints.
func (v *Verifier) Offline() *Verifier {
	copy := *v
	copy.revocationCheck = nil
	copy.revocation = nil
	return &copy
}

// SetOnlineRevocationCheck configures the privacy preference once before
// verification starts. The callback is consulted for every verification so
// changes in Settings take effect without restarting the application.
func (v *Verifier) SetOnlineRevocationCheck(enabled func() bool) {
	v.revocation = newRevocationChecker()
	v.revocationCheck = func(chains [][]*x509.Certificate) error {
		if enabled == nil || !enabled() {
			return errRevocationUnchecked
		}
		return v.revocation.checkChains(chains)
	}
}

// VerifyAndUnwrap detects S/MIME signed content, verifies the signature,
// caches the sender cert, and returns the verification result plus the
// unwrapped inner body (if any). If the message is not S/MIME signed or
// verification encounters a fatal parse error, it returns (nil, nil).
func (v *Verifier) VerifyAndUnwrap(raw []byte) (*SignatureResult, []byte) {
	// Parse the message to find Content-Type
	headerEnd := bytes.Index(raw, []byte("\r\n\r\n"))
	if headerEnd == -1 {
		headerEnd = bytes.Index(raw, []byte("\n\n"))
		if headerEnd == -1 {
			return nil, nil
		}
	}

	headers := raw[:headerEnd]
	ct := extractHeaderValue(headers, "Content-Type")
	if ct == "" {
		return nil, nil
	}

	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return nil, nil
	}

	// Handle multipart/signed with pkcs7-signature protocol
	if strings.EqualFold(mediaType, "multipart/signed") {
		protocol := params["protocol"]
		if !strings.EqualFold(protocol, "application/pkcs7-signature") &&
			!strings.EqualFold(protocol, "application/x-pkcs7-signature") {
			return nil, nil
		}
		return v.verifyMultipartSigned(raw, params)
	}

	// Handle application/pkcs7-mime (opaque signed)
	if strings.EqualFold(mediaType, "application/pkcs7-mime") ||
		strings.EqualFold(mediaType, "application/x-pkcs7-mime") {
		smimeType := params["smime-type"]
		if strings.EqualFold(smimeType, "signed-data") {
			return v.verifyOpaqueSigned(raw)
		}
	}

	return nil, nil
}

// verifyMultipartSigned handles clear-signed messages (multipart/signed)
func (v *Verifier) verifyMultipartSigned(raw []byte, params map[string]string) (*SignatureResult, []byte) {
	boundary := params["boundary"]
	if boundary == "" {
		return &SignatureResult{
			Status:       StatusInvalid,
			ErrorMessage: "missing boundary parameter",
		}, nil
	}

	// Find the body after headers
	headerEnd := bytes.Index(raw, []byte("\r\n\r\n"))
	bodyStart := headerEnd + 4
	if headerEnd == -1 {
		headerEnd = bytes.Index(raw, []byte("\n\n"))
		bodyStart = headerEnd + 2
	}
	if headerEnd == -1 {
		return &SignatureResult{
			Status:       StatusInvalid,
			ErrorMessage: "cannot find header/body boundary",
		}, nil
	}

	body := raw[bodyStart:]

	// RFC 2046 §5.1: Extract the raw bytes of the first body part.
	// For detached signature verification the signed content MUST be the
	// exact bytes between the opening boundary's trailing CRLF and the
	// CRLF that introduces the next boundary delimiter. We must NOT
	// re-parse the part headers because any re-serialization (e.g.
	// header reordering) would invalidate the signature.
	boundaryLine := []byte("--" + boundary)

	// Locate the opening boundary delimiter
	firstIdx := bytes.Index(body, boundaryLine)
	if firstIdx == -1 {
		return &SignatureResult{
			Status:       StatusInvalid,
			ErrorMessage: "cannot find opening boundary",
		}, nil
	}

	// Content starts right after the boundary line's CRLF
	contentStart := firstIdx + len(boundaryLine)
	if contentStart+2 <= len(body) && body[contentStart] == '\r' && body[contentStart+1] == '\n' {
		contentStart += 2
	} else if contentStart < len(body) && body[contentStart] == '\n' {
		contentStart++
	}

	// Find the next boundary delimiter. Per RFC 2046, the CRLF
	// preceding the delimiter line belongs to the boundary, not to the
	// encapsulated part.
	rest := body[contentStart:]
	delim := []byte("\r\n--" + boundary)
	endIdx := bytes.Index(rest, delim)
	if endIdx == -1 {
		// Try with bare LF
		delim = []byte("\n--" + boundary)
		endIdx = bytes.Index(rest, delim)
		if endIdx == -1 {
			return &SignatureResult{
				Status:       StatusInvalid,
				ErrorMessage: "cannot find closing boundary for signed part",
			}, nil
		}
	}

	signedContent := rest[:endIdx]

	// Extract the signature from the second part. We use
	// multipart.Reader here because the exact bytes of the signature
	// part are irrelevant — we only need the decoded PKCS#7 data.
	reader := multipart.NewReader(bytes.NewReader(body), boundary)

	// Skip the first part (NextPart consumes the previous part internally)
	if p, err := reader.NextPart(); err == nil {
		_, _ = io.Copy(io.Discard, p)
	}

	// Second part: the PKCS#7 detached signature
	sigPart, err := reader.NextPart()
	if err != nil {
		return &SignatureResult{
			Status:       StatusInvalid,
			ErrorMessage: fmt.Sprintf("failed to read signature part: %v", err),
		}, nil
	}
	sigBytes, err := io.ReadAll(sigPart)
	if err != nil {
		return &SignatureResult{
			Status:       StatusInvalid,
			ErrorMessage: fmt.Sprintf("failed to read signature bytes: %v", err),
		}, nil
	}

	// The signature part is typically base64-encoded per its
	// Content-Transfer-Encoding header.  Try parsing as raw DER first;
	// if that fails, base64-decode and retry.
	p7, err := pkcs7.Parse(sigBytes)
	if err != nil {
		// Strip whitespace/line breaks from base64 data
		cleaned := bytes.Map(func(r rune) rune {
			if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
				return -1
			}
			return r
		}, sigBytes)
		decoded, decErr := base64.StdEncoding.DecodeString(string(cleaned))
		if decErr != nil {
			return &SignatureResult{
				Status:       StatusInvalid,
				ErrorMessage: fmt.Sprintf("failed to parse PKCS#7 signature: %v", err),
			}, nil
		}
		p7, err = pkcs7.Parse(decoded)
		if err != nil {
			return &SignatureResult{
				Status:       StatusInvalid,
				ErrorMessage: fmt.Sprintf("failed to parse PKCS#7 signature after base64 decode: %v", err),
			}, nil
		}
	}

	// Attach the raw signed content for detached signature verification
	p7.Content = signedContent

	// Verify the signature
	result := v.verifyPKCS7(p7)

	return result, signedContent
}

// verifyOpaqueSigned handles opaque signed messages (application/pkcs7-mime)
func (v *Verifier) verifyOpaqueSigned(raw []byte) (*SignatureResult, []byte) {
	// Find body after headers
	headerEnd := bytes.Index(raw, []byte("\r\n\r\n"))
	bodyStart := headerEnd + 4
	if headerEnd == -1 {
		headerEnd = bytes.Index(raw, []byte("\n\n"))
		bodyStart = headerEnd + 2
	}
	if headerEnd == -1 {
		return &SignatureResult{
			Status:       StatusInvalid,
			ErrorMessage: "cannot find header/body boundary",
		}, nil
	}

	body := raw[bodyStart:]

	// Try parsing as DER first; if that fails, base64-decode and retry
	p7, err := pkcs7.Parse(body)
	if err != nil {
		cleaned := bytes.Map(func(r rune) rune {
			if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
				return -1
			}
			return r
		}, body)
		decoded, decErr := base64.StdEncoding.DecodeString(string(cleaned))
		if decErr != nil {
			return &SignatureResult{
				Status:       StatusInvalid,
				ErrorMessage: fmt.Sprintf("failed to parse PKCS#7 data: %v", err),
			}, nil
		}
		p7, err = pkcs7.Parse(decoded)
		if err != nil {
			return &SignatureResult{
				Status:       StatusInvalid,
				ErrorMessage: fmt.Sprintf("failed to parse PKCS#7 data after base64 decode: %v", err),
			}, nil
		}
	}

	result := v.verifyPKCS7(p7)

	// For opaque signed, the inner content is embedded in p7.Content
	return result, p7.Content
}

// verifyPKCS7 verifies a parsed PKCS#7 object and caches the signer cert
func (v *Verifier) verifyPKCS7(p7 *pkcs7.PKCS7) *SignatureResult {
	signers := signerCertificates(p7)
	if signers == nil {
		return &SignatureResult{
			Status:       StatusInvalid,
			ErrorMessage: "no certificate for signer",
		}
	}
	signer := signers[0]
	signerEmail, signerName := signerIdentity(signer)

	// Check the signature itself. pkcs7.Verify() does not check the
	// certificate chain; trust is evaluated separately below.
	if err := p7.Verify(); err != nil {
		return &SignatureResult{
			Status:       StatusInvalid,
			SignerEmail:  signerEmail,
			SignerName:   signerName,
			ErrorMessage: fmt.Sprintf("signature verification failed: %v", err),
		}
	}

	result := v.verifySigners(p7, signers, signerEmail, signerName)

	return result
}

// verifySigners requires every signer, not just the displayed one, to chain
// to a trusted root.
func (v *Verifier) verifySigners(p7 *pkcs7.PKCS7, signers []*x509.Certificate, signerEmail, signerName string) *SignatureResult {
	var revocationWarning, failure *SignatureResult
	for _, cert := range signers {
		err := v.verifySignerChain(p7, cert)
		if err == nil {
			continue
		}
		result := &SignatureResult{
			Status:       StatusUnknownSigner,
			SignerEmail:  signerEmail,
			SignerName:   signerName,
			ErrorMessage: fmt.Sprintf("unverified signer: %v", err),
			signerEmails: certEmails(signers[0]),
		}
		switch {
		case errors.Is(err, errCertificateRevoked):
			result.Status = StatusRevokedCert
			result.ErrorMessage = "signer certificate has been revoked"
		case errors.Is(err, errRevocationUnchecked):
			result.Status = StatusRevocationUnchecked
		case errors.Is(err, errRevocationUnknown):
			result.Status = StatusRevocationUnknown
		case time.Now().After(cert.NotAfter):
			result.Status = StatusExpiredCert
			result.ErrorMessage = "signer certificate has expired"
		case bytes.Equal(cert.RawIssuer, cert.RawSubject):
			result.Status = StatusSelfSigned
			result.ErrorMessage = "self-signed certificate"
		}
		if result.Status == StatusRevocationUnchecked || result.Status == StatusRevocationUnknown {
			revocationWarning = result
			continue
		}
		if failure == nil || result.Status == StatusRevokedCert {
			failure = result
		}
	}
	if failure != nil {
		return failure
	}
	if revocationWarning != nil {
		return revocationWarning
	}

	return &SignatureResult{
		Status:       StatusSigned,
		SignerEmail:  signerEmail,
		SignerName:   signerName,
		signerEmails: certEmails(signers[0]),
	}
}

// verifySignerChain keeps known revocation attached to the actual signer,
// including when a multi-signer message displays another signer first.
func (v *Verifier) verifySignerChain(p7 *pkcs7.PKCS7, cert *x509.Certificate) error {
	err := v.verifyChain(p7, cert)
	fingerprint := certificateFingerprint(cert.Raw)
	switch {
	case err == nil:
		if saveErr := v.store.SetSenderCertRevoked(fingerprint, false); saveErr != nil {
			v.log.Warn().Err(saveErr).Msg("Failed to clear cached sender revocation")
		}
	case errors.Is(err, errRevocationUnchecked) || errors.Is(err, errRevocationUnknown):
		if revoked, lookupErr := v.store.SenderCertRevoked(fingerprint); lookupErr == nil && revoked {
			err = errCertificateRevoked
		}
	}
	// Encryption eligibility follows this signer's trusted chain, independently
	// of the displayed aggregate status or another signer's failure.
	trusted := err == nil || errors.Is(err, errRevocationUnchecked) || errors.Is(err, errRevocationUnknown)
	email, _ := signerIdentity(cert)
	v.cacheSenderCert(cert, email, trusted)
	if errors.Is(err, errCertificateRevoked) {
		if saveErr := v.store.SetSenderCertRevoked(fingerprint, true); saveErr != nil {
			v.log.Warn().Err(saveErr).Msg("Failed to persist sender revocation")
		}
	}
	return err
}

// verifyChain verifies the signer certificate chains to a trusted root for
// email protection, using embedded certificates as intermediates. The chain
// is evaluated now: the signingTime attribute is chosen by the signer, so
// trusting it would let a holder of an expired certificate's key backdate
// new signatures into the certificate's validity period.
func (v *Verifier) verifyChain(p7 *pkcs7.PKCS7, signer *x509.Certificate) error {
	roots := v.roots
	if roots == nil {
		var err error
		if roots, err = x509.SystemCertPool(); err != nil {
			return fmt.Errorf("loading system roots: %w", err)
		}
	}

	intermediates := x509.NewCertPool()
	for _, cert := range p7.Certificates {
		if cert != signer {
			intermediates.AddCert(cert)
		}
	}

	chains, err := signer.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageEmailProtection},
	})
	if err != nil {
		return err
	}
	if v.revocationCheck == nil {
		return errRevocationUnchecked
	}
	return v.revocationCheck(chains)
}

// signerCertificates returns the certificate referenced by each
// SignerInfo's issuer and serial number, in SignerInfo order. It returns nil
// if there are no signers or any signer's certificate is not embedded.
func signerCertificates(p7 *pkcs7.PKCS7) []*x509.Certificate {
	if len(p7.Signers) == 0 {
		return nil
	}
	certs := make([]*x509.Certificate, 0, len(p7.Signers))
	for _, si := range p7.Signers {
		ias := si.IssuerAndSerialNumber
		var match *x509.Certificate
		for _, cert := range p7.Certificates {
			if cert.SerialNumber.Cmp(ias.SerialNumber) == 0 && bytes.Equal(cert.RawIssuer, ias.IssuerName.FullBytes) {
				match = cert
				break
			}
		}
		if match == nil {
			return nil
		}
		certs = append(certs, match)
	}
	return certs
}

// signerIdentity returns the email address and common name of the signer certificate
func signerIdentity(cert *x509.Certificate) (email, name string) {
	if len(cert.EmailAddresses) > 0 {
		email = cert.EmailAddresses[0]
	}
	return email, cert.Subject.CommonName
}

// oidEmailAddress is the PKCS#9 emailAddress subject attribute, which older
// certificates use instead of a subjectAltName rfc822Name.
var oidEmailAddress = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 1}

// certEmails returns every email address cert names.
func certEmails(cert *x509.Certificate) []string {
	emails := append([]string(nil), cert.EmailAddresses...)
	for _, name := range cert.Subject.Names {
		if email, ok := name.Value.(string); ok && name.Type.Equal(oidEmailAddress) {
			emails = append(emails, email)
		}
	}
	return emails
}

// cacheSenderCert stores the signer's certificate for future reference.
// trusted marks it as usable for encrypting to email.
func (v *Verifier) cacheSenderCert(cert *x509.Certificate, email string, trusted bool) {
	if email == "" {
		return
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})

	if err := v.store.CacheSenderCert(email, string(certPEM), trusted); err != nil {
		v.log.Warn().Err(err).Str("email", email).Msg("Failed to cache sender certificate")
	}
}

// extractHeaderValue extracts a header value from raw headers (case-insensitive)
func extractHeaderValue(headers []byte, name string) string {
	lines := strings.Split(string(headers), "\n")
	lowerName := strings.ToLower(name)

	for i, line := range lines {
		line = strings.TrimRight(line, "\r")
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}

		headerName := strings.ToLower(strings.TrimSpace(line[:colonIdx]))
		if headerName != lowerName {
			continue
		}

		value := strings.TrimSpace(line[colonIdx+1:])

		// Handle multi-line headers (continuation lines start with whitespace)
		for j := i + 1; j < len(lines); j++ {
			nextLine := strings.TrimRight(lines[j], "\r")
			if len(nextLine) == 0 {
				break
			}
			if nextLine[0] == ' ' || nextLine[0] == '\t' {
				value += " " + strings.TrimSpace(nextLine)
			} else {
				break
			}
		}

		return value
	}
	return ""
}

// IsSMIMEEncrypted checks if a Content-Type header indicates S/MIME encrypted content
func IsSMIMEEncrypted(contentType string) bool {
	if contentType == "" {
		return false
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}

	if strings.EqualFold(mediaType, "application/pkcs7-mime") ||
		strings.EqualFold(mediaType, "application/x-pkcs7-mime") {
		smimeType := params["smime-type"]
		return strings.EqualFold(smimeType, "enveloped-data")
	}

	return false
}

// IsSMIMESigned checks if a Content-Type header indicates S/MIME signed content
func IsSMIMESigned(contentType string) bool {
	if contentType == "" {
		return false
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}

	// multipart/signed with pkcs7 protocol
	if strings.EqualFold(mediaType, "multipart/signed") {
		protocol := params["protocol"]
		return strings.EqualFold(protocol, "application/pkcs7-signature") ||
			strings.EqualFold(protocol, "application/x-pkcs7-signature")
	}

	// application/pkcs7-mime with signed-data type
	if strings.EqualFold(mediaType, "application/pkcs7-mime") ||
		strings.EqualFold(mediaType, "application/x-pkcs7-mime") {
		smimeType := params["smime-type"]
		return strings.EqualFold(smimeType, "signed-data")
	}

	return false
}
