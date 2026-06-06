// Package tools holds the thin MCP handlers: they map inputs/outputs and call the service. No
// business logic. Tool descriptions are what the agent reads, so each states what it does, its
// scope, and its impact. The irreversible one, gmail_send, is impact:high and gated at the gateway.
package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/scottylabs/scottylabs-agent/mcp-servers/google-workspace/internal/domain"
	"github.com/scottylabs/scottylabs-agent/mcp-servers/google-workspace/internal/service"
)

type sheetsReadInput struct {
	SpreadsheetID string `json:"spreadsheet_id" jsonschema:"the spreadsheet id"`
	Range         string `json:"range" jsonschema:"A1 notation range, e.g. Sheet1!A1:D100"`
}

type calendarListInput struct {
	CalendarID string `json:"calendar_id" jsonschema:"calendar id; defaults to primary"`
	TimeMin    string `json:"time_min" jsonschema:"RFC3339 lower bound (optional)"`
	TimeMax    string `json:"time_max" jsonschema:"RFC3339 upper bound (optional)"`
	Max        int64  `json:"max" jsonschema:"max events to return (optional)"`
}

type calendarCreateInput struct {
	CalendarID  string   `json:"calendar_id" jsonschema:"calendar id; defaults to primary"`
	Summary     string   `json:"summary" jsonschema:"event title"`
	Description string   `json:"description" jsonschema:"event description (optional)"`
	Location    string   `json:"location" jsonschema:"event location (optional)"`
	Start       string   `json:"start" jsonschema:"RFC3339 start time"`
	End         string   `json:"end" jsonschema:"RFC3339 end time"`
	Attendees   []string `json:"attendees" jsonschema:"attendee emails (optional)"`
}

type emailInput struct {
	To      []string `json:"to" jsonschema:"recipient emails"`
	Cc      []string `json:"cc" jsonschema:"cc emails (optional)"`
	Subject string   `json:"subject" jsonschema:"email subject"`
	Body    string   `json:"body" jsonschema:"email body (plain text)"`
}

type driveReadInput struct {
	Query string `json:"query" jsonschema:"Drive query, e.g. name contains 'poster' (optional)"`
	Max   int64  `json:"max" jsonschema:"max files to return (optional)"`
}

type idOutput struct {
	ID string `json:"id"`
}

// Register wires the Workspace tools onto the server. The service is injected (DI).
func Register(s *mcp.Server, svc *service.Workspace) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "sheets_read",
		Description: "Read a range of values from a Google Sheet. Read-only. scope: google.sheets.read, impact: read.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in sheetsReadInput) (*mcp.CallToolResult, domain.SheetData, error) {
		out, err := svc.ReadSheet(ctx, in.SpreadsheetID, in.Range)
		return nil, out, err
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "calendar_list",
		Description: "List Google Calendar events in an optional time window. Read-only. scope: google.calendar.read, impact: read.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in calendarListInput) (*mcp.CallToolResult, []domain.CalendarEvent, error) {
		out, err := svc.ListEvents(ctx, in.CalendarID, in.TimeMin, in.TimeMax, in.Max)
		return nil, out, err
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "calendar_create_event",
		Description: "Create a Google Calendar event. Reversible write. scope: google.calendar.write, impact: write.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in calendarCreateInput) (*mcp.CallToolResult, domain.CalendarEvent, error) {
		out, err := svc.CreateEvent(ctx, in.CalendarID, domain.CalendarEvent{
			Summary: in.Summary, Description: in.Description, Location: in.Location,
			Start: in.Start, End: in.End, Attendees: in.Attendees,
		})
		return nil, out, err
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "gmail_draft",
		Description: "Create a Gmail draft (does NOT send). Reversible. scope: google.gmail.draft, impact: write.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in emailInput) (*mcp.CallToolResult, idOutput, error) {
		id, err := svc.DraftEmail(ctx, domain.EmailDraft{To: in.To, Cc: in.Cc, Subject: in.Subject, Body: in.Body})
		return nil, idOutput{ID: id}, err
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "gmail_send",
		Description: "Send an email as the agent. IRREVERSIBLE — requires human approval at the gateway. " +
			"scope: google.gmail.send, impact: high.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in emailInput) (*mcp.CallToolResult, idOutput, error) {
		id, err := svc.SendEmail(ctx, domain.EmailDraft{To: in.To, Cc: in.Cc, Subject: in.Subject, Body: in.Body})
		return nil, idOutput{ID: id}, err
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "drive_read",
		Description: "List Google Drive file metadata matching a query. Read-only. scope: google.drive.read, impact: read.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in driveReadInput) (*mcp.CallToolResult, []domain.DriveFile, error) {
		out, err := svc.ListDriveFiles(ctx, in.Query, in.Max)
		return nil, out, err
	})
}
