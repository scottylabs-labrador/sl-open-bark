package service_test

import (
	"context"
	"testing"

	"github.com/scottylabs/scottylabs-agent/mcp-servers/google-workspace/internal/domain"
	"github.com/scottylabs/scottylabs-agent/mcp-servers/google-workspace/internal/service"
)

// fakeGoogle implements service.Google with fixtures and call recording — the "fake Google client"
// the WP requires so tests make no live calls.
type fakeGoogle struct {
	sheet      domain.SheetData
	events     []domain.CalendarEvent
	drive      []domain.DriveFile
	createdEvt domain.CalendarEvent
	draftCalls int
	sendCalls  int
	lastCal    string
}

func (f *fakeGoogle) ReadSheet(_ context.Context, id, rng string) (domain.SheetData, error) {
	f.sheet.SpreadsheetID, f.sheet.Range = id, rng
	return f.sheet, nil
}
func (f *fakeGoogle) ListEvents(_ context.Context, calendarID, _, _ string, _ int64) ([]domain.CalendarEvent, error) {
	f.lastCal = calendarID
	return f.events, nil
}
func (f *fakeGoogle) CreateEvent(_ context.Context, calendarID string, e domain.CalendarEvent) (domain.CalendarEvent, error) {
	f.lastCal = calendarID
	e.ID = "evt-1"
	f.createdEvt = e
	return e, nil
}
func (f *fakeGoogle) CreateDraft(_ context.Context, _ domain.EmailDraft) (string, error) {
	f.draftCalls++
	return "draft-1", nil
}
func (f *fakeGoogle) SendEmail(_ context.Context, _ domain.EmailDraft) (string, error) {
	f.sendCalls++
	return "msg-1", nil
}
func (f *fakeGoogle) ListDriveFiles(_ context.Context, _ string, _ int64) ([]domain.DriveFile, error) {
	return f.drive, nil
}

func TestReadSheetReturnsFixture(t *testing.T) {
	fg := &fakeGoogle{sheet: domain.SheetData{Rows: [][]string{{"name", "amount"}, {"Alice", "20"}}}}
	w := service.New(fg)
	got, err := w.ReadSheet(context.Background(), "sheet123", "A1:B2")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 2 || got.Rows[1][0] != "Alice" || got.SpreadsheetID != "sheet123" {
		t.Fatalf("unexpected sheet data: %+v", got)
	}
	if _, err := w.ReadSheet(context.Background(), "", "A1"); err == nil {
		t.Fatal("empty spreadsheet id should error")
	}
}

func TestListEventsDefaultsToPrimary(t *testing.T) {
	fg := &fakeGoogle{events: []domain.CalendarEvent{{Summary: "Sync"}}}
	w := service.New(fg)
	if _, err := w.ListEvents(context.Background(), "", "", "", 10); err != nil {
		t.Fatal(err)
	}
	if fg.lastCal != "primary" {
		t.Fatalf("empty calendar should default to primary, got %q", fg.lastCal)
	}
}

func TestCreateEventValidatesBeforeCalling(t *testing.T) {
	fg := &fakeGoogle{}
	w := service.New(fg)
	if _, err := w.CreateEvent(context.Background(), "", domain.CalendarEvent{Start: "x", End: "y"}); err == nil {
		t.Fatal("event without summary should be rejected before calling Google")
	}
	if fg.createdEvt.ID != "" {
		t.Fatal("Google must not be called when validation fails")
	}
	ev, err := w.CreateEvent(context.Background(), "", domain.CalendarEvent{
		Summary: "Sync", Start: "2026-06-10T10:00:00Z", End: "2026-06-10T11:00:00Z",
	})
	if err != nil || ev.ID != "evt-1" || fg.lastCal != "primary" {
		t.Fatalf("create event failed: ev=%+v err=%v", ev, err)
	}
}

func TestDraftAndSendValidate(t *testing.T) {
	fg := &fakeGoogle{}
	w := service.New(fg)

	if _, err := w.DraftEmail(context.Background(), domain.EmailDraft{Subject: "Hi"}); err == nil {
		t.Fatal("draft with no recipient should error")
	}
	id, err := w.DraftEmail(context.Background(), domain.EmailDraft{To: []string{"a@cmu.edu"}, Subject: "Hi", Body: "x"})
	if err != nil || id != "draft-1" || fg.draftCalls != 1 {
		t.Fatalf("draft failed: id=%q err=%v", id, err)
	}

	// Send is the high-impact path (gated at the gateway); the server still validates.
	msg, err := w.SendEmail(context.Background(), domain.EmailDraft{To: []string{"a@cmu.edu"}, Subject: "Hi"})
	if err != nil || msg != "msg-1" || fg.sendCalls != 1 {
		t.Fatalf("send failed: msg=%q err=%v", msg, err)
	}
}

func TestListDriveFiles(t *testing.T) {
	fg := &fakeGoogle{drive: []domain.DriveFile{{ID: "1", Name: "poster.png", MimeType: "image/png"}}}
	w := service.New(fg)
	files, err := w.ListDriveFiles(context.Background(), "name contains 'poster'", 5)
	if err != nil || len(files) != 1 || files[0].Name != "poster.png" {
		t.Fatalf("drive list failed: %+v err=%v", files, err)
	}
}
