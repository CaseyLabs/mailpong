package apiactions

import (
	"context"
	"errors"
	"slices"

	"github.com/caseylabs/mailven/internal/apiwrite"
	"github.com/caseylabs/mailven/internal/gmailapi"
	"github.com/caseylabs/mailven/internal/graphapi"
	"github.com/caseylabs/mailven/internal/message"
)

// Verdict is what a read of the provider says about a queued action.
type Verdict int

const (
	// Unverifiable: the read cannot tell whether the action took effect.
	Unverifiable Verdict = iota
	// Matches: the provider already shows the action's result.
	Matches
	// Differs: the provider still shows the state before the action.
	Differs
)

// ErrGone means the message no longer exists at the provider.
var ErrGone = errors.New("message gone at provider")

// Result carries what a successful apply changed about the message identity.
type Result struct {
	// NewRemoteID is set when the move gave the message a new ID.
	NewRemoteID string
	// Containers is the message's containers afterwards, when the provider says.
	Containers []string
}

// Remote applies and verifies one queued action at a provider. Apply errors are
// classified with apiwrite.OutcomeOf; a missing message is ErrGone.
type Remote interface {
	Apply(ctx context.Context, a *message.APIAction) (Result, error)
	Verify(ctx context.Context, a *message.APIAction) (Verdict, error)
}

func gone(err error) error {
	if errors.Is(err, gmailapi.ErrWriteGone) || errors.Is(err, graphapi.ErrWriteGone) ||
		errors.Is(err, gmailapi.ErrMessageGone) || errors.Is(err, graphapi.ErrMessageGone) {
		return ErrGone
	}
	return err
}

// Gmail applies actions as label changes. Flags map to UNREAD and STARRED;
// trash uses the trash verbs; every other move adds the destination label and
// removes the source label.
type Gmail struct {
	Writer *gmailapi.Writer
	Reader *gmailapi.Reader
}

func (g *Gmail) Apply(ctx context.Context, a *message.APIAction) (Result, error) {
	p := a.Payload
	var err error
	switch a.Op {
	case message.APIActionRead:
		// Read removes UNREAD.
		add, remove := gmailLabel(gmailapi.LabelUnread, !p.Value)
		_, err = g.Writer.Modify(ctx, a.RemoteID, add, remove)
	case message.APIActionStar:
		add, remove := gmailLabel(gmailapi.LabelStarred, p.Value)
		_, err = g.Writer.Modify(ctx, a.RemoteID, add, remove)
	case message.APIActionMove:
		err = g.move(ctx, a)
	default:
		return Result{}, &apiwrite.Error{Outcome: apiwrite.Rejected, Err: errors.New("unsupported api action " + a.Op)}
	}
	return Result{}, gone(err)
}

func gmailLabel(label string, present bool) (add, remove []string) {
	if present {
		return []string{label}, nil
	}
	return nil, []string{label}
}

func (g *Gmail) move(ctx context.Context, a *message.APIAction) error {
	p := a.Payload
	if p.Trash {
		_, err := g.Writer.Trash(ctx, a.RemoteID)
		return err
	}
	if p.FromTrash {
		if _, err := g.Writer.Untrash(ctx, a.RemoteID); err != nil {
			return err
		}
	}
	var add, remove []string
	if p.Dest != "" {
		add = append(add, p.Dest)
	}
	if p.Source != "" && !p.FromTrash {
		remove = append(remove, p.Source)
	}
	if len(add) == 0 && len(remove) == 0 {
		return nil
	}
	_, err := g.Writer.Modify(ctx, a.RemoteID, add, remove)
	return err
}

func (g *Gmail) Verify(ctx context.Context, a *message.APIAction) (Verdict, error) {
	m, err := g.Reader.GetMinimal(ctx, a.RemoteID)
	if err != nil {
		return Unverifiable, gone(err)
	}
	has := func(l string) bool { return slices.Contains(m.LabelIDs, l) }
	p := a.Payload
	switch a.Op {
	case message.APIActionRead:
		return verdict(has(gmailapi.LabelUnread) == !p.Value), nil
	case message.APIActionStar:
		return verdict(has(gmailapi.LabelStarred) == p.Value), nil
	case message.APIActionMove:
		switch {
		case p.Trash:
			return verdict(has(gmailapi.LabelTrash)), nil
		case p.Dest != "":
			return verdict(has(p.Dest) && !has(gmailapi.LabelTrash) && (p.Source == "" || !has(p.Source))), nil
		case p.Source != "":
			return verdict(!has(p.Source)), nil
		}
	}
	return Unverifiable, nil
}

func verdict(matches bool) Verdict {
	if matches {
		return Matches
	}
	return Differs
}

// Graph applies actions as message updates and folder moves. Move
// destinations are folder IDs or well-known names; verifying a move needs the
// destination and source as folder IDs.
type Graph struct {
	Writer *graphapi.Writer
	Reader *graphapi.Reader
}

func (g *Graph) Apply(ctx context.Context, a *message.APIAction) (Result, error) {
	p := a.Payload
	switch a.Op {
	case message.APIActionRead:
		return Result{}, gone(g.Writer.Update(ctx, a.RemoteID, graphapi.Patch{IsRead: &p.Value}))
	case message.APIActionStar:
		return Result{}, gone(g.Writer.Update(ctx, a.RemoteID, graphapi.Patch{Flagged: &p.Value}))
	case message.APIActionMove:
		moved, err := g.Writer.Move(ctx, a.RemoteID, p.Dest)
		if err != nil {
			return Result{}, gone(err)
		}
		res := Result{NewRemoteID: moved.ID}
		if moved.ParentFolderID != "" {
			res.Containers = []string{moved.ParentFolderID}
		}
		return res, nil
	}
	return Result{}, &apiwrite.Error{Outcome: apiwrite.Rejected, Err: errors.New("unsupported api action " + a.Op)}
}

func (g *Graph) Verify(ctx context.Context, a *message.APIAction) (Verdict, error) {
	m, err := g.Reader.GetMeta(ctx, a.RemoteID)
	if err != nil {
		return Unverifiable, gone(err)
	}
	p := a.Payload
	switch a.Op {
	case message.APIActionRead:
		return verdict(m.IsRead == p.Value), nil
	case message.APIActionStar:
		return verdict(m.Flagged == p.Value), nil
	case message.APIActionMove:
		switch m.ParentFolderID {
		case p.Dest:
			return Matches, nil
		case p.Source:
			return Differs, nil
		}
	}
	return Unverifiable, nil
}
