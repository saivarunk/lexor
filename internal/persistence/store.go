package persistence

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	p "github.com/varun/lexor/pkg/pluginapi"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct {
	DB       *sql.DB
	Location *time.Location
}

func Open(file string) (*Store, error) {
	db, err := sql.Open("sqlite", file)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, q := range []string{"PRAGMA foreign_keys=ON", "PRAGMA journal_mode=WAL", "PRAGMA busy_timeout=5000", "CREATE TABLE IF NOT EXISTS schema_migrations(version TEXT PRIMARY KEY)"} {
		if _, err = db.Exec(q); err != nil {
			db.Close()
			return nil, err
		}
	}
	files, _ := migrations.ReadDir("migrations")
	for _, f := range files {
		var n int
		if err = db.QueryRow("SELECT count(*) FROM schema_migrations WHERE version=?", f.Name()).Scan(&n); err != nil {
			db.Close()
			return nil, err
		}
		if n > 0 {
			continue
		}
		b, _ := migrations.ReadFile("migrations/" + f.Name())
		tx, e := db.Begin()
		if e != nil {
			db.Close()
			return nil, e
		}
		_, e = tx.Exec(string(b))
		if e == nil {
			_, e = tx.Exec("INSERT INTO schema_migrations(version) VALUES(?)", f.Name())
		}
		if e != nil {
			tx.Rollback()
			db.Close()
			return nil, e
		}
		if e = tx.Commit(); e != nil {
			db.Close()
			return nil, e
		}
	}

	// Pin the workspace's original IANA timezone so DST works and travel does not
	// silently reassign historical activity. Fixed offsets are an explicit fallback.
	loc, name := initialLocation()
	var saved string
	err = db.QueryRow("SELECT value FROM settings WHERE key='activity_timezone'").Scan(&saved)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = db.Exec("INSERT INTO settings(key,value) VALUES('activity_timezone',?)", name)
	} else if err == nil {
		if strings.HasPrefix(saved, "offset:") {
			var offset int
			if _, e := fmt.Sscanf(saved, "offset:%d", &offset); e != nil {
				db.Close()
				return nil, e
			}
			loc = time.FixedZone(saved, offset)
		} else {
			loc, err = time.LoadLocation(saved)
		}
	}
	if err != nil {
		db.Close()
		return nil, err
	}

	return &Store{DB: db, Location: loc}, nil
}
func (s *Store) Close() error { return s.DB.Close() }
func (s *Store) Draft(ctx context.Context, id, language string) (p.Draft, error) {
	d := p.Draft{ProblemID: id, Language: language}
	var at string
	err := s.DB.QueryRowContext(ctx, "SELECT source,updated_at FROM code_drafts WHERE problem_id=? AND language=?", id, language).Scan(&d.Source, &at)
	if err != nil {
		return d, err
	}
	timestamp, e := time.Parse(time.RFC3339Nano, at)
	if e != nil {
		return d, e
	}
	d.UpdatedAt = &timestamp
	return d, nil
}
func (s *Store) SaveDraft(ctx context.Context, d p.Draft) error {
	_, err := s.DB.ExecContext(ctx, "INSERT INTO code_drafts(problem_id,language,source,updated_at) VALUES(?,?,?,?) ON CONFLICT(problem_id,language) DO UPDATE SET source=excluded.source,updated_at=excluded.updated_at", d.ProblemID, d.Language, d.Source, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
func (s *Store) Record(ctx context.Context, r p.ExecutionResult) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	at := r.CreatedAt.UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, "INSERT INTO executions(id,problem_id,language,source,mode,status,created_at,result_json) VALUES(?,?,?,?,?,?,?,?)", r.ID, r.ProblemID, r.Language, r.Source, r.Mode, r.Status, at, string(b)); err != nil {
		return err
	}
	if r.Mode == "submit" && r.Status != "Sandbox Unavailable" && r.Status != "Internal Error" {
		var first sql.NullString
		err = tx.QueryRowContext(ctx, "SELECT first_solved_at FROM problem_progress WHERE problem_id=?", r.ProblemID).Scan(&first)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		accepted := r.Status == "Accepted"
		status := "attempted"
		var solved any
		if accepted {
			status = "solved"
			solved = at
		}
		best := 0.0
		if accepted {
			best = r.DurationMS
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO problem_progress(problem_id,status,attempts,best_duration_ms,first_attempted_at,last_attempted_at,first_solved_at,last_solved_at) VALUES(?,?,1,?,?,?,?,?) ON CONFLICT(problem_id) DO UPDATE SET status=CASE WHEN problem_progress.status='solved' THEN 'solved' ELSE excluded.status END,attempts=problem_progress.attempts+1,best_duration_ms=CASE WHEN excluded.status='solved' AND (problem_progress.best_duration_ms=0 OR excluded.best_duration_ms<problem_progress.best_duration_ms) THEN excluded.best_duration_ms ELSE problem_progress.best_duration_ms END,last_attempted_at=excluded.last_attempted_at,first_solved_at=COALESCE(problem_progress.first_solved_at,excluded.first_solved_at),last_solved_at=COALESCE(excluded.last_solved_at,problem_progress.last_solved_at)`, r.ProblemID, status, best, at, at, solved, solved)
		if err != nil {
			return err
		}
		if accepted {
			newSolve := 0
			if !first.Valid {
				newSolve = 1
			}
			_, err = tx.ExecContext(ctx, "INSERT INTO daily_activity(date,count,new_solves) VALUES(?,1,?) ON CONFLICT(date) DO UPDATE SET count=count+1,new_solves=new_solves+excluded.new_solves", r.CreatedAt.In(s.Location).Format("2006-01-02"), newSolve)
			if err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
func (s *Store) Executions(ctx context.Context, mode, id string, limit int) ([]p.ExecutionResult, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT result_json FROM executions WHERE (?='' OR mode=?) AND (?='' OR problem_id=?) ORDER BY created_at DESC LIMIT ?", mode, mode, id, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []p.ExecutionResult{}
	for rows.Next() {
		var b string
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		var r p.ExecutionResult
		if err = json.Unmarshal([]byte(b), &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *Store) Execution(ctx context.Context, id string) (p.ExecutionResult, error) {
	var b string
	var r p.ExecutionResult
	err := s.DB.QueryRowContext(ctx, "SELECT result_json FROM executions WHERE id=?", id).Scan(&b)
	if err != nil {
		return r, err
	}
	err = json.Unmarshal([]byte(b), &r)
	return r, err
}
func (s *Store) Progress(ctx context.Context) ([]p.Progress, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT problem_id,status,attempts,best_duration_ms,first_attempted_at,last_attempted_at,first_solved_at,last_solved_at FROM problem_progress ORDER BY problem_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []p.Progress{}
	for rows.Next() {
		var v p.Progress
		var first, last string
		var fs, ls sql.NullString
		if err = rows.Scan(&v.ProblemID, &v.Status, &v.Attempts, &v.BestDurationMS, &first, &last, &fs, &ls); err != nil {
			return nil, err
		}
		v.FirstAttemptedAt, _ = time.Parse(time.RFC3339Nano, first)
		v.LastAttemptedAt, _ = time.Parse(time.RFC3339Nano, last)
		if fs.Valid {
			t, _ := time.Parse(time.RFC3339Nano, fs.String)
			v.FirstSolvedAt = &t
		}
		if ls.Valid {
			t, _ := time.Parse(time.RFC3339Nano, ls.String)
			v.LastSolvedAt = &t
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) Activity(ctx context.Context) ([]p.Activity, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT date,count,new_solves FROM daily_activity ORDER BY date")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []p.Activity{}
	for rows.Next() {
		var a p.Activity
		if err = rows.Scan(&a.Date, &a.Count, &a.NewSolves); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func Streaks(activity []p.Activity, now time.Time) (current, longest int) {
	days := map[string]bool{}
	dates := []string{}
	for _, a := range activity {
		if a.Count > 0 && !days[a.Date] {
			days[a.Date] = true
			dates = append(dates, a.Date)
		}
	}
	sort.Strings(dates)
	run := 0
	var prev time.Time
	for _, d := range dates {
		t, err := time.ParseInLocation("2006-01-02", d, now.Location())
		if err != nil {
			continue
		}
		if !prev.IsZero() && prev.AddDate(0, 0, 1).Format("2006-01-02") == d {
			run++
		} else {
			run = 1
		}
		if run > longest {
			longest = run
		}
		prev = t
	}
	start := now
	if !days[start.Format("2006-01-02")] {
		start = start.AddDate(0, 0, -1)
	}
	for days[start.Format("2006-01-02")] {
		current++
		start = start.AddDate(0, 0, -1)
	}
	return
}
func (s *Store) Stats(ctx context.Context, problems []p.ProblemView) (p.Stats, error) {
	result := p.Stats{Total: len(problems), Difficulty: map[string]int{"easy": 0, "medium": 0, "hard": 0}}
	progress, err := s.Progress(ctx)
	if err != nil {
		return result, err
	}
	index := map[string]p.ProblemView{}
	for _, pr := range problems {
		index[pr.ID] = pr
	}
	for _, v := range progress {
		if pr, ok := index[v.ProblemID]; ok {
			result.Attempted++
			if v.Status == "solved" {
				result.Solved++
				result.Difficulty[pr.Difficulty]++
			}
		}
	}
	result.Activity, err = s.Activity(ctx)
	if err != nil {
		return result, err
	}
	now := time.Now().In(s.Location)
	result.CurrentStreak, result.LongestStreak = Streaks(result.Activity, now)
	weekday := (int(now.Weekday()) + 6) % 7
	start := now.AddDate(0, 0, -weekday).Format("2006-01-02")
	for _, a := range result.Activity {
		if a.Date >= start && a.Date <= now.Format("2006-01-02") {
			result.SolvedThisWeek += a.NewSolves
		}
	}
	result.Recent, err = s.Executions(ctx, "submit", "", 5)
	for i := range result.Recent {
		result.Recent[i].Source = ""
	}
	return result, err
}

func initialLocation() (*time.Location, string) {
	candidates := []string{os.Getenv("TZ")}
	if path, err := filepath.EvalSymlinks("/etc/localtime"); err == nil {
		if _, name, ok := strings.Cut(path, "/zoneinfo/"); ok {
			candidates = append(candidates, name)
		}
	}
	if b, err := os.ReadFile("/etc/timezone"); err == nil {
		candidates = append(candidates, strings.TrimSpace(string(b)))
	}
	for _, name := range candidates {
		if name != "" && name != "Local" {
			if loc, err := time.LoadLocation(name); err == nil {
				return loc, name
			}
		}
	}
	_, offset := time.Now().Zone()
	name := fmt.Sprintf("offset:%d", offset)
	return time.FixedZone(name, offset), name
}
