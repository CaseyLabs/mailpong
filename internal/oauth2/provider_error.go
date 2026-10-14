package oauth2

import (
	"errors"
	"strings"
)

// ProviderError is an OAuth error response from an authorization or token
// endpoint, keeping the provider's code for classification.
type ProviderError struct {
	Code        string
	Description string
	message     string
}

func (e *ProviderError) Error() string { return e.message }

// Is keeps ErrInvalidGrant matching a rejected refresh token.
func (e *ProviderError) Is(target error) bool {
	return target == ErrInvalidGrant && e.Code == "invalid_grant"
}

// MailAPIFailure says what a user must change after a failed API authorization.
type MailAPIFailure string

const (
	MailAPIFailureNone           MailAPIFailure = ""
	MailAPIFailureReauthorize    MailAPIFailure = "reauthorize"     // token rejected or revoked
	MailAPIFailurePolicy         MailAPIFailure = "policy"          // administrator consent or access policy
	MailAPIFailureResource       MailAPIFailure = "resource"        // scope or resource unknown to the provider/tenant
	MailAPIFailurePartialConsent MailAPIFailure = "partial-consent" // granted permissions lack a required capability
	MailAPIFailureOther          MailAPIFailure = "other"
)

// Microsoft AADSTS codes: 65001/65004/90094 consent, 53003/50105/50158/530003
// conditional access or assignment, 500011/650052/70011 resource or scope.
var (
	policyCodes   = []string{"AADSTS65001", "AADSTS65004", "AADSTS90094", "AADSTS53003", "AADSTS50105", "AADSTS50158", "AADSTS530003"}
	resourceCodes = []string{"AADSTS500011", "AADSTS650052", "AADSTS70011"}
)

// ClassifyMailAPIError maps a provider failure to an actionable category. A
// non-provider error (network, local) is "other": consent cannot fix it.
func ClassifyMailAPIError(err error) MailAPIFailure {
	if err == nil {
		return MailAPIFailureNone
	}
	var pe *ProviderError
	if !errors.As(err, &pe) {
		return MailAPIFailureOther
	}
	for _, code := range resourceCodes {
		if strings.Contains(pe.Description, code) {
			return MailAPIFailureResource
		}
	}
	for _, code := range policyCodes {
		if strings.Contains(pe.Description, code) {
			return MailAPIFailurePolicy
		}
	}
	switch pe.Code {
	case "invalid_scope":
		return MailAPIFailureResource
	case "access_denied", "consent_required", "interaction_required", "admin_policy_enforced", "unauthorized_client":
		return MailAPIFailurePolicy
	case "invalid_grant":
		return MailAPIFailureReauthorize
	}
	return MailAPIFailureOther
}

// NewProviderError builds a ProviderError for a failure that did not come from
// a token endpoint (for example an API HTTP status), so it can be classified
// with ClassifyMailAPIError.
func NewProviderError(code, description string) *ProviderError {
	return &ProviderError{Code: code, Description: description, message: code + ": " + description}
}
