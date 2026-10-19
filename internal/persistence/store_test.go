package persistence

import (
	"context"
	p "github.com/varun/lexor/pkg/pluginapi"
	"path/filepath"
	"testing"
	"time"
)

func TestPersistenceTransactionsAndIdempotentMigrations(t *testing.T) {
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "progress.db")
	s, err := Open(file)
	if err != nil {
		t.Fatal(err)
	}
	s.Location = time.UTC
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	if err = s.SaveDraft(ctx, p.Draft{ProblemID: "arrays.two-sum", Language: "python", Source: "draft"}); err != nil {
		t.Fatal(err)
	}
	for i, mode := range []string{"run", "submit", "submit"} {
		r := p.ExecutionResult{ID: mode + time.Duration(i).String(), ProblemID: "arrays.two-sum", Language: "python", Source: "accepted", Status: "Accepted", Mode: mode, DurationMS: 4, CreatedAt: now, Results: []p.TestResult{}}
		if err = s.Record(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	progress, err := s.Progress(ctx)
	if err != nil || len(progress) != 1 || progress[0].Attempts != 2 || progress[0].Status != "solved" {
		t.Fatalf("progress: %+v %v", progress, err)
	}
	activity, err := s.Activity(ctx)
	if err != nil || len(activity) != 1 || activity[0].Count != 2 || activity[0].NewSolves != 1 {
		t.Fatalf("activity: %+v %v", activity, err)
	}
	s.Close()
	s, err = Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d, err := s.Draft(ctx, "arrays.two-sum", "python")
	if err != nil || d.Source != "draft" {
		t.Fatal("draft lost across restart")
	}
	submissions, err := s.Executions(ctx, "submit", "", 100)
	if err != nil || len(submissions) != 2 {
		t.Fatal("submissions lost")
	}
	if err = s.Record(ctx, submissions[0]); err == nil {
		t.Fatal("duplicate execution accepted")
	}
	progress, _ = s.Progress(ctx)
	if progress[0].Attempts != 2 {
		t.Fatal("failed transaction changed progress")
	}
}
func TestStreaksCalendarRules(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name             string
		dates            []string
		today            string
		current, longest int
	}{
		{"DST transition", []string{"2026-03-07", "2026-03-08", "2026-03-09"}, "2026-03-09", 3, 3},
		{"yesterday still active", []string{"2026-10-17", "2026-10-18"}, "2026-10-19", 2, 2},
		{"gap resets", []string{"2026-10-15", "2026-10-16"}, "2026-10-19", 0, 2},
		{"duplicates", []string{"2026-10-19", "2026-10-18", "2026-10-18"}, "2026-10-19", 2, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			activity := []p.Activity{}
			for _, d := range tc.dates {
				activity = append(activity, p.Activity{Date: d, Count: 1})
			}
			now, _ := time.ParseInLocation("2006-01-02", tc.today, loc)
			current, longest := Streaks(activity, now)
			if current != tc.current || longest != tc.longest {
				t.Fatalf("got %d,%d", current, longest)
			}
		})
	}
}

func TestPreferencesAndTimezoneSurviveRestart(t *testing.T) {
	t.Setenv("TZ", "America/New_York")
	file := filepath.Join(t.TempDir(), "settings.db")
	s, err := Open(file)
	if err != nil {
		t.Fatal(err)
	}
	v := p.Preferences{Theme: "light", EditorTheme: "dark", EditorFontSize: 16}
	if err = s.SavePreferences(context.Background(), v); err != nil {
		t.Fatal(err)
	}
	if s.Location.String() != "America/New_York" {
		t.Fatal(s.Location)
	}
	s.Close()
	t.Setenv("TZ", "Asia/Kolkata")
	s, err = Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Preferences(context.Background())
	if err != nil || got != v || s.Location.String() != "America/New_York" {
		t.Fatalf("settings changed: %+v %s %v", got, s.Location, err)
	}
}
