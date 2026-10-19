package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	p "github.com/varun/lexor/pkg/pluginapi"
	"time"
)

func (s *Store) Preferences(ctx context.Context) (p.Preferences, error) {
	v := p.Preferences{Theme: "dark", EditorTheme: "dark", EditorFontSize: 14}
	var raw string
	err := s.DB.QueryRowContext(ctx, "SELECT value FROM settings WHERE key='preferences'").Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	err = json.Unmarshal([]byte(raw), &v)
	return v, err
}
func (s *Store) SavePreferences(ctx context.Context, v p.Preferences) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, "INSERT INTO settings(key,value) VALUES('preferences',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(raw))
	return err
}

func (s *Store) RecordInstallation(ctx context.Context, source, digest string, ids []string) error {
	b, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, "INSERT INTO registry_installations(source,source_digest,imported_at,problem_ids) VALUES(?,?,?,?)", source, digest, time.Now().UTC().Format(time.RFC3339Nano), string(b))
	return err
}
