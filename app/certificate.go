package app

import (
	"github.com/caseylabs/mailpong/internal/certificate"
	"github.com/caseylabs/mailpong/internal/logging"
)

// ============================================================================
// Certificate Trust API - Exposed to frontend via Wails bindings
// ============================================================================

// AcceptCertificate accepts a certificate for the given host.
// If permanent is true, the certificate is stored in the database.
// If permanent is false, the certificate is only trusted for the current session.
func (a *App) AcceptCertificate(host string, info certificate.CertificateInfo, permanent bool) error {
	log := logging.WithComponent("app.certificate")

	if permanent {
		log.Info().
			Str("host", host).
			Str("fingerprint", info.Fingerprint).
			Msg("Permanently accepting certificate")
		return a.certStore.AcceptPermanently(host, &info)
	}

	log.Info().
		Str("host", host).
		Str("fingerprint", info.Fingerprint).
		Msg("Accepting certificate for session")
	return a.certStore.AcceptSession(host, info.Fingerprint)
}

// GetTrustedCertificates returns permanently trusted certificates for the given hosts
func (a *App) GetTrustedCertificates(hosts []string) ([]*certificate.CertificateInfo, error) {
	return a.certStore.GetByHosts(hosts)
}

// ServerCertificateCheck is the result of CheckServerCertificate.
type ServerCertificateCheck struct {
	CertificateRequired bool                         `json:"certificateRequired"`
	Host                string                       `json:"host,omitempty"`
	Certificate         *certificate.CertificateInfo `json:"certificate,omitempty"`
}

// CheckServerCertificate connects to an HTTPS server URL (following redirects)
// and reports the first certificate the trust store does not accept, with the
// exact host that presented it. Setup flows whose connection failed (CardDAV,
// CalDAV) call it to offer the certificate-accept prompt, then pass Host to
// AcceptCertificate so trust is pinned to that host before retrying.
func (a *App) CheckServerCertificate(serverURL string) (ServerCertificateCheck, error) {
	log := logging.WithComponent("app.certificate")

	res, err := certificate.Probe(a.ctx, serverURL, a.certStore)
	if err != nil {
		log.Debug().Err(err).Msg("Server certificate check failed")
		return ServerCertificateCheck{}, err
	}
	if res.Info == nil {
		return ServerCertificateCheck{}, nil
	}
	log.Info().
		Str("host", res.Host).
		Str("fingerprint", res.Info.Fingerprint).
		Msg("Untrusted server certificate")
	return ServerCertificateCheck{CertificateRequired: true, Host: res.Host, Certificate: res.Info}, nil
}

// RemoveTrustedCertificate removes a certificate from the trust store by fingerprint
func (a *App) RemoveTrustedCertificate(fingerprint string) error {
	log := logging.WithComponent("app.certificate")
	log.Info().Str("fingerprint", fingerprint).Msg("Removing trusted certificate")
	return a.certStore.Remove(fingerprint)
}
