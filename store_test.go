package shrt

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// Links from before chosen names existed were all generated, so a
// database from then keeps answering their URLs with their codes.
func TestMigrationMarksOlderLinksGenerated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shrt.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		migrations[0],
		`INSERT INTO links (code, url, created_at) VALUES ('k3P9qa', 'https://example.com/old', '2026-10-01T08:00:00Z')`,
		`PRAGMA user_version = 1`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	db.Close()

	store := openTestStore(t, path)
	link, err := store.GeneratedFor(context.Background(), "https://example.com/old")
	if err != nil {
		t.Fatalf("GeneratedFor: %v", err)
	}
	if link.Code != "k3P9qa" {
		t.Errorf("code %q, want k3P9qa", link.Code)
	}
}

// Two calls for the same URL can both find no generated code; only the
// first may then insert one.
func TestInsertGeneratedKeepsOneCodePerURL(t *testing.T) {
	store := openTestStore(t, filepath.Join(t.TempDir(), "shrt.db"))
	ctx := context.Background()
	link := Link{Code: "aaaaaa", URL: "https://example.com", CreatedAt: testNow}

	for _, tc := range []struct {
		insert func(context.Context, Link) (bool, error)
		code   string
		want   bool
	}{
		{store.InsertGenerated, "aaaaaa", true},
		{store.InsertGenerated, "bbbbbb", false}, // the URL has a generated code
		{store.InsertNamed, "ferien", true},      // a name doesn't care
		{store.InsertGenerated, "aaaaaa", false}, // the code is taken
		{store.InsertNamed, "aaaaaa", false},
	} {
		link.Code = tc.code
		if got, err := tc.insert(ctx, link); err != nil || got != tc.want {
			t.Errorf("inserting %s: %v, %v; want %v", tc.code, got, err, tc.want)
		}
	}

	other := Link{Code: "cccccc", URL: "https://example.com/other", CreatedAt: testNow}
	if got, err := store.InsertGenerated(ctx, other); err != nil || !got {
		t.Errorf("another URL: %v, %v; want true", got, err)
	}
}
