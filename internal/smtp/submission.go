package smtp

import (
	"errors"
	"fmt"
	"net/textproto"
)

// Outcome describes what is known about an SMTP submission.
type Outcome string

const (
	OutcomeNotSubmitted      Outcome = "not_submitted"
	OutcomeRejectedTemporary Outcome = "rejected_temporary"
	OutcomeRejectedPermanent Outcome = "rejected_permanent"
	OutcomeAccepted          Outcome = "accepted"
	OutcomeUnknown           Outcome = "unknown"
)

// RecipientResult records the server's response to one RCPT TO command.
type RecipientResult struct {
	Address  string `json:"address"`
	Accepted bool   `json:"accepted"`
	Code     int    `json:"code,omitempty"`
	Response string `json:"response,omitempty"`
}

// SubmissionResult records the submission state and the point at which it was
// determined.
type SubmissionResult struct {
	Outcome    Outcome           `json:"outcome"`
	Stage      string            `json:"stage"`
	Recipients []RecipientResult `json:"recipients,omitempty"`
}

// SubmissionError carries the classified result alongside its underlying SMTP
// or persistence error.
type SubmissionError struct {
	Result SubmissionResult
	Err    error
}

func (e *SubmissionError) Error() string { return e.Err.Error() }
func (e *SubmissionError) Unwrap() error { return e.Err }

func responseCode(err error) (int, string, bool) {
	var protocolErr *textproto.Error
	if errors.As(err, &protocolErr) {
		return protocolErr.Code, protocolErr.Msg, true
	}
	return 0, "", false
}

func rejectedOutcome(err error) (Outcome, int, string, bool) {
	code, response, ok := responseCode(err)
	if !ok {
		return "", code, response, false
	}
	switch {
	case code >= 400 && code < 500:
		return OutcomeRejectedTemporary, code, response, true
	case code >= 500 && code < 600:
		return OutcomeRejectedPermanent, code, response, true
	default:
		return "", code, response, false
	}
}

func submissionError(result SubmissionResult, err error) error {
	if err == nil {
		return nil
	}
	return &SubmissionError{Result: result, Err: fmt.Errorf("%w", err)}
}
