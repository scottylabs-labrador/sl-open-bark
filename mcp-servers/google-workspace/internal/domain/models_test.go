package domain

import "testing"

func TestValidateEmailDraft(t *testing.T) {
	cases := []struct {
		name  string
		draft EmailDraft
		ok    bool
	}{
		{"valid", EmailDraft{To: []string{"a@cmu.edu"}, Subject: "Hi", Body: "x"}, true},
		{"no recipient", EmailDraft{Subject: "Hi"}, false},
		{"blank recipient", EmailDraft{To: []string{"  "}, Subject: "Hi"}, false},
		{"no subject", EmailDraft{To: []string{"a@cmu.edu"}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateEmailDraft(c.draft)
			if c.ok && err != nil {
				t.Fatalf("want valid, got %v", err)
			}
			if !c.ok && err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}

func TestValidateEvent(t *testing.T) {
	cases := []struct {
		name  string
		event CalendarEvent
		ok    bool
	}{
		{"valid", CalendarEvent{Summary: "Sync", Start: "2026-06-10T10:00:00Z", End: "2026-06-10T11:00:00Z"}, true},
		{"no summary", CalendarEvent{Start: "2026-06-10T10:00:00Z", End: "2026-06-10T11:00:00Z"}, false},
		{"no end", CalendarEvent{Summary: "Sync", Start: "2026-06-10T10:00:00Z"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateEvent(c.event)
			if c.ok && err != nil {
				t.Fatalf("want valid, got %v", err)
			}
			if !c.ok && err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}
