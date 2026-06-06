// Package domain holds the Google Workspace MCP's typed models and pure-function validation. No I/O
// lives here. The messy Google API access happens in the clients layer; this layer is deterministic
// and trivially testable.
package domain

import (
	"fmt"
	"strings"
)

// SheetData is a read range from a spreadsheet.
type SheetData struct {
	SpreadsheetID string     `json:"spreadsheet_id"`
	Range         string     `json:"range"`
	Rows          [][]string `json:"rows"`
}

// CalendarEvent is a calendar event. Times are RFC3339 strings (the agent and Google both speak it).
type CalendarEvent struct {
	ID          string   `json:"id,omitempty"`
	Summary     string   `json:"summary"`
	Description string   `json:"description,omitempty"`
	Location    string   `json:"location,omitempty"`
	Start       string   `json:"start"` // RFC3339
	End         string   `json:"end"`   // RFC3339
	Attendees   []string `json:"attendees,omitempty"`
}

// EmailDraft is an outbound email. Drafting is reversible (write); sending is irreversible (high).
type EmailDraft struct {
	To      []string `json:"to"`
	Cc      []string `json:"cc,omitempty"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
}

// DriveFile is a file's metadata from Drive (read-only listing).
type DriveFile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	MimeType     string `json:"mime_type"`
	ModifiedTime string `json:"modified_time,omitempty"`
}

// ValidateEmailDraft checks an email has at least one recipient and a subject. Pure function.
func ValidateEmailDraft(d EmailDraft) error {
	if len(nonEmpty(d.To)) == 0 {
		return fmt.Errorf("email must have at least one recipient")
	}
	if strings.TrimSpace(d.Subject) == "" {
		return fmt.Errorf("email must have a subject")
	}
	return nil
}

// ValidateEvent checks an event has a summary and both start and end times. Pure function.
func ValidateEvent(e CalendarEvent) error {
	if strings.TrimSpace(e.Summary) == "" {
		return fmt.Errorf("event must have a summary")
	}
	if strings.TrimSpace(e.Start) == "" || strings.TrimSpace(e.End) == "" {
		return fmt.Errorf("event must have start and end times (RFC3339)")
	}
	return nil
}

func nonEmpty(values []string) []string {
	var out []string
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}
