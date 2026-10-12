package certificate

import "fmt"

// CertificateInfo holds display-friendly certificate details
type CertificateInfo struct {
	Host        string   `json:"host,omitempty"`
	Subject     string   `json:"subject"`
	Issuer      string   `json:"issuer"`
	Fingerprint string   `json:"fingerprint"`
	NotBefore   string   `json:"notBefore"`
	NotAfter    string   `json:"notAfter"`
	DNSNames    []string `json:"dnsNames"`
	IsExpired   bool     `json:"isExpired"`
	ErrorReason string   `json:"errorReason"`
}

// Error is returned when a TLS certificate cannot be verified
type Error struct {
	Host   string
	token  string
	Info   *CertificateInfo
	Reason string
}

func (e *Error) Error() string {
	message := e.UserMessage()
	if e.token != "" {
		message += " [certificate-check:" + e.token + "]"
	}
	return message
}

func (e *Error) UserMessage() string {
	return fmt.Sprintf("untrusted certificate: %s (fingerprint: %s)", e.Reason, e.Info.Fingerprint)
}
