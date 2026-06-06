// Package service orchestrates the Google Workspace use cases. It depends on the Google interface
// it declares here (Go idiom: interface at the consumer) and on pure-function validation in the
// domain layer. Concrete Google access lives in the clients package and is injected.
package service

import (
	"context"
	"fmt"

	"github.com/scottylabs/scottylabs-agent/mcp-servers/google-workspace/internal/domain"
)

// Google is the set of Workspace operations this service needs. The real client (clients package)
// and a fake (tests) both satisfy it. Read operations are first-class; the only irreversible one is
// SendEmail, which the gateway gates as impact:high.
type Google interface {
	ReadSheet(ctx context.Context, spreadsheetID, rng string) (domain.SheetData, error)
	ListEvents(ctx context.Context, calendarID, timeMin, timeMax string, max int64) ([]domain.CalendarEvent, error)
	CreateEvent(ctx context.Context, calendarID string, e domain.CalendarEvent) (domain.CalendarEvent, error)
	CreateDraft(ctx context.Context, d domain.EmailDraft) (draftID string, err error)
	SendEmail(ctx context.Context, d domain.EmailDraft) (messageID string, err error)
	ListDriveFiles(ctx context.Context, query string, max int64) ([]domain.DriveFile, error)
}

// Workspace is the use-case orchestrator. Construct with New and inject a Google client.
type Workspace struct {
	google Google
}

func New(g Google) *Workspace { return &Workspace{google: g} }

// ReadSheet reads a range from a spreadsheet (impact: read).
func (w *Workspace) ReadSheet(ctx context.Context, spreadsheetID, rng string) (domain.SheetData, error) {
	if spreadsheetID == "" {
		return domain.SheetData{}, fmt.Errorf("spreadsheet_id is required")
	}
	return w.google.ReadSheet(ctx, spreadsheetID, rng)
}

// ListEvents lists events in a window (impact: read). calendarID defaults to "primary".
func (w *Workspace) ListEvents(ctx context.Context, calendarID, timeMin, timeMax string, max int64) ([]domain.CalendarEvent, error) {
	if calendarID == "" {
		calendarID = "primary"
	}
	return w.google.ListEvents(ctx, calendarID, timeMin, timeMax, max)
}

// CreateEvent creates a calendar event (impact: write). Validates before touching Google.
func (w *Workspace) CreateEvent(ctx context.Context, calendarID string, e domain.CalendarEvent) (domain.CalendarEvent, error) {
	if err := domain.ValidateEvent(e); err != nil {
		return domain.CalendarEvent{}, err
	}
	if calendarID == "" {
		calendarID = "primary"
	}
	return w.google.CreateEvent(ctx, calendarID, e)
}

// DraftEmail creates a Gmail draft (impact: write). Draft only — it does not send.
func (w *Workspace) DraftEmail(ctx context.Context, d domain.EmailDraft) (string, error) {
	if err := domain.ValidateEmailDraft(d); err != nil {
		return "", err
	}
	return w.google.CreateDraft(ctx, d)
}

// SendEmail sends an email (impact: HIGH — irreversible, gated at the gateway). The server still
// validates, but the human-approval guarantee is enforced by the gateway, not here.
func (w *Workspace) SendEmail(ctx context.Context, d domain.EmailDraft) (string, error) {
	if err := domain.ValidateEmailDraft(d); err != nil {
		return "", err
	}
	return w.google.SendEmail(ctx, d)
}

// ListDriveFiles lists Drive files matching a query (impact: read).
func (w *Workspace) ListDriveFiles(ctx context.Context, query string, max int64) ([]domain.DriveFile, error) {
	return w.google.ListDriveFiles(ctx, query, max)
}
