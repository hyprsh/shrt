package shrt

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned for a code that has no link.
var ErrNotFound = errors.New("link not found")

// Store keeps links in a SQLite file.
type Store struct {
	db *sql.DB
}

// migrations[i] takes the schema from version i to version i+1, as recorded
// in SQLite's user_version. Append to change the schema; never edit one that
// has shipped.
var migrations = []string{
	`CREATE TABLE links (
		code       TEXT PRIMARY KEY,
		url        TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`,
	// generated tells a code shrt drew apart from a chosen name, which can
	// look the same (decision 10). Every link until now was generated.
	`ALTER TABLE links ADD COLUMN generated INTEGER NOT NULL DEFAULT 0;
	UPDATE links SET generated = 1;
	CREATE INDEX links_generated_url ON links (url) WHERE generated`,
}

// OpenStore opens the SQLite file at path, creating it if need be, and
// brings its schema up to date.
func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	// One connection serialises writes, which is plenty for shrt and
	// rules out SQLITE_BUSY between its own connections.
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating %s: %w", path, err)
	}
	return s, nil
}

func (s *Store) migrate() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version > len(migrations) {
		return fmt.Errorf("schema version %d is newer than this shrt (%d)", version, len(migrations))
	}
	for _, m := range migrations[version:] {
		if _, err := tx.Exec(m); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, len(migrations))); err != nil {
		return err
	}
	return tx.Commit()
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// Ping checks that the database file can be read.
func (s *Store) Ping(ctx context.Context) error {
	var n int
	return s.db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema`).Scan(&n)
}

// InsertNamed stores link under its chosen name unless the name is taken, and
// reports whether it did. An existing link is never overwritten.
func (s *Store) InsertNamed(ctx context.Context, link Link) (inserted bool, err error) {
	return insertedOne(s.db.ExecContext(ctx,
		`INSERT INTO links (code, url, created_at, generated) VALUES (?, ?, ?, 0)
		ON CONFLICT (code) DO NOTHING`,
		link.Code, link.URL, formatCreatedAt(link.CreatedAt)))
}

// InsertGenerated stores link under the code shrt generated for it, and
// reports whether it did. It doesn't when the code is taken or when the URL
// already has a generated code, which GeneratedFor then finds. Checking and
// inserting in one statement keeps two calls for the same URL from both
// getting a code.
func (s *Store) InsertGenerated(ctx context.Context, link Link) (inserted bool, err error) {
	// SQLite needs the WHERE to tell ON CONFLICT from a join's ON.
	return insertedOne(s.db.ExecContext(ctx,
		`INSERT INTO links (code, url, created_at, generated)
		SELECT ?, ?, ?, 1 WHERE NOT EXISTS (SELECT 1 FROM links WHERE url = ? AND generated)
		ON CONFLICT (code) DO NOTHING`,
		link.Code, link.URL, formatCreatedAt(link.CreatedAt), link.URL))
}

// formatCreatedAt is how created_at is stored.
func formatCreatedAt(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func insertedOne(res sql.Result, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// Get returns the link with code, or ErrNotFound. Codes are case-sensitive.
func (s *Store) Get(ctx context.Context, code string) (Link, error) {
	return scanLink(s.db.QueryRowContext(ctx, `SELECT code, url, created_at FROM links WHERE code = ?`, code))
}

// GeneratedFor returns the link with the code shrt generated for url, or
// ErrNotFound when url has none; links under a chosen name don't count.
// URLs match exactly. Should a URL have several generated codes, as it can
// from before shrt kept to one, the oldest wins.
func (s *Store) GeneratedFor(ctx context.Context, url string) (Link, error) {
	return scanLink(s.db.QueryRowContext(ctx,
		`SELECT code, url, created_at FROM links WHERE url = ? AND generated
		ORDER BY created_at, rowid LIMIT 1`, url))
}

func scanLink(row *sql.Row) (Link, error) {
	var link Link
	var createdAt string
	err := row.Scan(&link.Code, &link.URL, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Link{}, ErrNotFound
	}
	if err != nil {
		return Link{}, err
	}
	link.CreatedAt, err = parseCreatedAt(createdAt)
	return link, err
}

// parseCreatedAt reads the RFC3339 text that formatCreatedAt writes.
func parseCreatedAt(text string) (time.Time, error) {
	return time.Parse(time.RFC3339, text)
}

// List returns every link, newest first. Links made in the same second are
// ordered by insertion, the later one first.
func (s *Store) List(ctx context.Context) ([]Link, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT code, url, created_at FROM links ORDER BY created_at DESC, rowid DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	links := []Link{}
	for rows.Next() {
		var link Link
		var createdAt string
		if err := rows.Scan(&link.Code, &link.URL, &createdAt); err != nil {
			return nil, err
		}
		if link.CreatedAt, err = parseCreatedAt(createdAt); err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

// Delete removes the link with code, or returns ErrNotFound. The code is
// free to be used again afterwards.
func (s *Store) Delete(ctx context.Context, code string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM links WHERE code = ?`, code)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
