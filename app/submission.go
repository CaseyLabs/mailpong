package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/textproto"

	"github.com/caseylabs/mailven/internal/draft"
	"github.com/caseylabs/mailven/internal/smtp"
	"github.com/caseylabs/mailven/internal/submission"
)

func (ops *composeOps) prepareSubmission(accountID string, msg smtp.ComposeMessage, draftID string) (*submission.Attempt, error) {
	if ops.submissions == nil {
		return nil, fmt.Errorf("submission journal unavailable; message was not submitted")
	}
	return ops.submissions.Begin(accountID, draftID, msg)
}

func applySubmissionResult(a *submission.Attempt, result smtp.SubmissionResult) {
	a.Outcome, a.Stage = string(result.Outcome), result.Stage
	b, _ := json.Marshal(result.Recipients)
	a.Recipients = string(b)
}

// finishAttempt separates transport outcomes from cleanup errors/panics. A
// positive final reply can never become a generic "failed, retry" response.
func (ops *composeOps) finishAttempt(a *submission.Attempt, sendErr *error) {
	if r := recover(); r != nil {
		if a.Outcome == "accepted" {
			a.Warnings += fmt.Sprintf("Post-acceptance cleanup interrupted: %v", r)
		} else {
			*sendErr = fmt.Errorf("submission interrupted: %v", r)
		}
	}
	// Nested calls see the review error from the inner call for unknown
	// outcomes; keep the underlying error already recorded.
	if *sendErr != nil && (a.Outcome != "unknown" || a.Error == "") {
		a.Error = (*sendErr).Error()
	}
	if *sendErr != nil {
		if a.Outcome == "not_submitted" {
			var rejection *textproto.Error
			if errors.As(*sendErr, &rejection) {
				switch {
				case rejection.Code >= 400 && rejection.Code < 500:
					a.Outcome = "rejected_temporary"
				case rejection.Code >= 500 && rejection.Code < 600:
					a.Outcome = "rejected_permanent"
				}
			}
		}
	}
	if err := ops.submissions.Update(a); err != nil {
		// An accepted result was already committed before cleanup. For other
		// states leave the pre-DATA checkpoint authoritative if updating fails.
		if a.Outcome == "accepted" {
			sendLog().Warn().Err(err).Msg("Failed to record cleanup warning")
		} else {
			*sendErr = fmt.Errorf("submission journal update failed: %w; %v", err, submission.ReviewError(a))
		}
	}
	if a.Outcome == "unknown" {
		*sendErr = submission.ReviewError(a)
	}
	if a.Outcome == "accepted" {
		*sendErr = nil
	}
}

// GetSubmissionReview is read-only. A review token is accepted only on a new
// explicit send action; merely opening a draft never resubmits it.
func (a *App) GetSubmissionReview(accountID string, msg smtp.ComposeMessage, draftID string) (*submission.Attempt, error) {
	return a.composeOps.submissions.Review(accountID, draftID, msg)
}
func (c *ComposerApp) GetSubmissionReview(accountID string, msg smtp.ComposeMessage, draftID string) (*submission.Attempt, error) {
	return c.composeOps.submissions.Review(accountID, draftID, msg)
}
func (a *App) GetSubmissionAttempts(accountID string) ([]*submission.Attempt, error) {
	return a.composeOps.submissions.List(accountID)
}
func (a *App) GetChatSubmissionReview(r ChatReply) (*submission.Attempt, error) {
	msg, err := a.buildChatReply(r)
	if err != nil {
		return nil, err
	}
	return a.composeOps.submissions.Review(r.AccountID, r.DraftID, *msg)
}

func (a *App) saveSubmissionDraft(accountID string, msg smtp.ComposeMessage, draftID string) (*draft.Draft, error) {
	var d *draft.Draft
	if draftID != "" {
		var err error
		d, err = a.draftStore.Get(draftID)
		if err != nil {
			return nil, err
		}
		if d == nil {
			return nil, fmt.Errorf("submission draft not found")
		}
	}
	return a.draftOps.persistRecoveryDraft(accountID, msg, d)
}

// A submission recovery copy must not downgrade requested draft encryption.
func (ops *draftOps) persistRecoveryDraft(accountID string, msg smtp.ComposeMessage, d *draft.Draft) (*draft.Draft, error) {
	enc, err := ops.encryptDraftBody(accountID, msg.From.Address, msg)
	if err != nil {
		return nil, err
	}
	if (msg.EncryptMessage && !enc.encrypted) || (msg.PGPEncryptMessage && !msg.EncryptMessage && !enc.pgpEncrypted) {
		return nil, fmt.Errorf("recovery draft could not be encrypted; message was not submitted")
	}
	return ops.saveDraftToDB(accountID, d, msg, enc)
}

func (a *App) GetSubmissionAttempt(id string) (*submission.Attempt, error) {
	return a.composeOps.submissions.Get(id)
}
