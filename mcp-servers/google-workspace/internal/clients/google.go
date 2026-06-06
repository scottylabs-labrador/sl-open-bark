// Package clients holds the concrete Google Workspace client — the side-effecting edge. It satisfies
// the service.Google interface structurally (so this package does not import service, avoiding an
// import cycle). It authenticates with a service account using domain-wide delegation, impersonating
// the agent's scottylabs.org account within narrowly listed scopes (design Section 8.1). Credentials
// come from the environment; none are in code.
package clients

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"github.com/scottylabs/scottylabs-agent/mcp-servers/google-workspace/internal/domain"
)

// Scopes is the tight, reviewed scope list this server's tools need (design Section 8.1). The
// Workspace admin grants exactly these to the delegation, and the list is reviewed when a tool is
// added.
var Scopes = []string{
	sheets.SpreadsheetsReadonlyScope,
	calendar.CalendarEventsScope,
	gmail.GmailComposeScope,
	gmail.GmailSendScope,
	drive.DriveReadonlyScope,
}

// Google is the concrete Workspace client.
type Google struct {
	sheets   *sheets.Service
	calendar *calendar.Service
	gmail    *gmail.Service
	drive    *drive.Service
}

// NewGoogle builds a client from a service-account JSON key, impersonating subject (e.g.
// agent@scottylabs.org) via domain-wide delegation, limited to Scopes.
func NewGoogle(ctx context.Context, saJSON []byte, subject string) (*Google, error) {
	cfg, err := google.JWTConfigFromJSON(saJSON, Scopes...)
	if err != nil {
		return nil, fmt.Errorf("clients: parse service account: %w", err)
	}
	cfg.Subject = subject // domain-wide delegation: act as this Workspace user
	ts := cfg.TokenSource(ctx)
	opt := option.WithTokenSource(ts)

	sheetsSvc, err := sheets.NewService(ctx, opt)
	if err != nil {
		return nil, fmt.Errorf("clients: sheets: %w", err)
	}
	calendarSvc, err := calendar.NewService(ctx, opt)
	if err != nil {
		return nil, fmt.Errorf("clients: calendar: %w", err)
	}
	gmailSvc, err := gmail.NewService(ctx, opt)
	if err != nil {
		return nil, fmt.Errorf("clients: gmail: %w", err)
	}
	driveSvc, err := drive.NewService(ctx, opt)
	if err != nil {
		return nil, fmt.Errorf("clients: drive: %w", err)
	}
	return &Google{sheets: sheetsSvc, calendar: calendarSvc, gmail: gmailSvc, drive: driveSvc}, nil
}

// ReadSheet reads a range of values from a spreadsheet.
func (g *Google) ReadSheet(ctx context.Context, spreadsheetID, rng string) (domain.SheetData, error) {
	resp, err := g.sheets.Spreadsheets.Values.Get(spreadsheetID, rng).Context(ctx).Do()
	if err != nil {
		return domain.SheetData{}, fmt.Errorf("clients: read sheet: %w", err)
	}
	rows := make([][]string, 0, len(resp.Values))
	for _, row := range resp.Values {
		cells := make([]string, 0, len(row))
		for _, c := range row {
			cells = append(cells, fmt.Sprintf("%v", c))
		}
		rows = append(rows, cells)
	}
	return domain.SheetData{SpreadsheetID: spreadsheetID, Range: resp.Range, Rows: rows}, nil
}

// ListEvents lists events in [timeMin, timeMax] (RFC3339; either may be empty).
func (g *Google) ListEvents(ctx context.Context, calendarID, timeMin, timeMax string, max int64) ([]domain.CalendarEvent, error) {
	call := g.calendar.Events.List(calendarID).SingleEvents(true).OrderBy("startTime")
	if timeMin != "" {
		call = call.TimeMin(timeMin)
	}
	if timeMax != "" {
		call = call.TimeMax(timeMax)
	}
	if max > 0 {
		call = call.MaxResults(max)
	}
	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("clients: list events: %w", err)
	}
	out := make([]domain.CalendarEvent, 0, len(resp.Items))
	for _, item := range resp.Items {
		out = append(out, toDomainEvent(item))
	}
	return out, nil
}

// CreateEvent inserts a calendar event.
func (g *Google) CreateEvent(ctx context.Context, calendarID string, e domain.CalendarEvent) (domain.CalendarEvent, error) {
	ev := &calendar.Event{
		Summary:     e.Summary,
		Description: e.Description,
		Location:    e.Location,
		Start:       &calendar.EventDateTime{DateTime: e.Start},
		End:         &calendar.EventDateTime{DateTime: e.End},
	}
	for _, a := range e.Attendees {
		ev.Attendees = append(ev.Attendees, &calendar.EventAttendee{Email: a})
	}
	created, err := g.calendar.Events.Insert(calendarID, ev).Context(ctx).Do()
	if err != nil {
		return domain.CalendarEvent{}, fmt.Errorf("clients: create event: %w", err)
	}
	return toDomainEvent(created), nil
}

// CreateDraft creates a Gmail draft (reversible).
func (g *Google) CreateDraft(ctx context.Context, d domain.EmailDraft) (string, error) {
	draft, err := g.gmail.Users.Drafts.Create("me", &gmail.Draft{
		Message: &gmail.Message{Raw: encodeMessage(d)},
	}).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("clients: create draft: %w", err)
	}
	return draft.Id, nil
}

// SendEmail sends an email (irreversible — gated impact:high at the gateway).
func (g *Google) SendEmail(ctx context.Context, d domain.EmailDraft) (string, error) {
	msg, err := g.gmail.Users.Messages.Send("me", &gmail.Message{Raw: encodeMessage(d)}).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("clients: send email: %w", err)
	}
	return msg.Id, nil
}

// ListDriveFiles lists Drive file metadata matching a query.
func (g *Google) ListDriveFiles(ctx context.Context, query string, max int64) ([]domain.DriveFile, error) {
	call := g.drive.Files.List().Fields("files(id,name,mimeType,modifiedTime)")
	if query != "" {
		call = call.Q(query)
	}
	if max > 0 {
		call = call.PageSize(max)
	}
	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("clients: list drive files: %w", err)
	}
	out := make([]domain.DriveFile, 0, len(resp.Files))
	for _, f := range resp.Files {
		out = append(out, domain.DriveFile{ID: f.Id, Name: f.Name, MimeType: f.MimeType, ModifiedTime: f.ModifiedTime})
	}
	return out, nil
}

func toDomainEvent(e *calendar.Event) domain.CalendarEvent {
	var attendees []string
	for _, a := range e.Attendees {
		attendees = append(attendees, a.Email)
	}
	return domain.CalendarEvent{
		ID: e.Id, Summary: e.Summary, Description: e.Description, Location: e.Location,
		Start: eventTime(e.Start), End: eventTime(e.End), Attendees: attendees,
	}
}

func eventTime(t *calendar.EventDateTime) string {
	if t == nil {
		return ""
	}
	if t.DateTime != "" {
		return t.DateTime
	}
	return t.Date
}

// encodeMessage renders an EmailDraft as a base64url-encoded RFC822 message for the Gmail API.
func encodeMessage(d domain.EmailDraft) string {
	var b strings.Builder
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(d.To, ", "))
	if len(d.Cc) > 0 {
		fmt.Fprintf(&b, "Cc: %s\r\n", strings.Join(d.Cc, ", "))
	}
	fmt.Fprintf(&b, "Subject: %s\r\n", d.Subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n")
	b.WriteString(d.Body)
	return base64.RawURLEncoding.EncodeToString([]byte(b.String()))
}
