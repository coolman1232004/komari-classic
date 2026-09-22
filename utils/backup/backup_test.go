package backup

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	dir := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(dir, "komari.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
CREATE TABLE users (uuid TEXT, username TEXT, passwd TEXT, two_factor TEXT);
CREATE TABLE clients (uuid TEXT, token TEXT);
CREATE TABLE configs (key TEXT, value TEXT);
CREATE TABLE sessions (session TEXT, uuid TEXT);
CREATE TABLE records (client TEXT, time TEXT);
CREATE TABLE ping_records (client TEXT, time TEXT);
INSERT INTO users VALUES ('admin', 'saved-user', 'saved-hash', 'saved-mfa');
INSERT INTO clients VALUES ('node', 'saved-agent-token');
INSERT INTO sessions VALUES ('old-browser-session', 'admin');
INSERT INTO records VALUES ('node', '2026-09-01');
INSERT INTO ping_records VALUES ('node', '2026-09-01');`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.txt"), []byte("backup settings"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "empty-theme-directory"), 0700); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, dir, "v1.2.5-fix2-security.3"); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func put(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func assertContents(t *testing.T, path, expected string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil || string(b) != expected {
		t.Fatalf("%s: contents=%q err=%v", path, b, err)
	}
}

func TestRestorePreservesRollbackAndRevokesSessions(t *testing.T) {
	data := t.TempDir()
	put(t, filepath.Join(data, "komari.db"), []byte("original database"))
	put(t, filepath.Join(data, "komari.db-wal"), []byte("original WAL"))
	put(t, filepath.Join(data, PendingName), fixture(t))
	if err := ApplyPending(data, filepath.Join(data, "komari.db")); err != nil {
		t.Fatal(err)
	}
	assertContents(t, filepath.Join(data, "settings.txt"), "backup settings")
	if info, err := os.Stat(filepath.Join(data, "empty-theme-directory")); err != nil || !info.IsDir() {
		t.Fatal("empty directory was lost")
	}
	if _, err := os.Stat(filepath.Join(data, "komari.db-wal")); !os.IsNotExist(err) {
		t.Fatal("stale WAL survived")
	}
	db, err := openDatabase(filepath.Join(data, "komari.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for query, want := range map[string]string{
		"SELECT count(*) FROM sessions":     "0",
		"SELECT token FROM clients":         "saved-agent-token",
		"SELECT passwd FROM users":          "saved-hash",
		"SELECT two_factor FROM users":      "saved-mfa",
		"SELECT count(*) FROM records":      "1",
		"SELECT count(*) FROM ping_records": "1",
	} {
		var got string
		if err := db.QueryRow(query).Scan(&got); err != nil || got != want {
			t.Fatalf("%s: %q %v", query, got, err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(data, HistoryName))
	if err != nil || len(entries) != 1 {
		t.Fatalf("missing persistent recovery directory: %v", err)
	}
	previous := filepath.Join(data, HistoryName, entries[0].Name(), "previous")
	assertContents(t, filepath.Join(previous, "komari.db"), "original database")
	assertContents(t, filepath.Join(previous, "komari.db-wal"), "original WAL")
	for _, name := range []string{PendingName, JournalName, ManifestName} {
		if _, err := os.Lstat(filepath.Join(data, name)); !os.IsNotExist(err) {
			t.Fatalf("unexpected %s: %v", name, err)
		}
	}
	if err := ApplyPending(data, filepath.Join(data, "komari.db")); err != nil {
		t.Fatalf("ordinary restart failed: %v", err)
	}
}

func editArchive(t *testing.T, original []byte, edit func(map[string][]byte)) []byte {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(original), int64(len(original)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range r.File {
		in, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name], err = io.ReadAll(in)
		in.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	edit(files)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, data := range files {
		out, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := out.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRejectedBackupsNeverTouchLiveData(t *testing.T) {
	good := fixture(t)
	cases := map[string]func(map[string][]byte){
		"legacy":           func(f map[string][]byte) { delete(f, ManifestName); f["komari-backup-markup"] = []byte("old backup") },
		"missing-database": func(f map[string][]byte) { delete(f, "komari.db") },
		"corrupt-data":     func(f map[string][]byte) { f["settings.txt"] = []byte("wrong settings") },
		"extra-database":   func(f map[string][]byte) { f["metrics.db"] = []byte("foreign data") },
		"stale-wal":        func(f map[string][]byte) { f["komari.db-wal"] = []byte("old WAL") },
		"reserved-journal": func(f map[string][]byte) { f[JournalName] = []byte("x") },
		"reserved-history": func(f map[string][]byte) { f[HistoryName+"/file"] = []byte("x") },
		"traversal":        func(f map[string][]byte) { f["../escaped"] = []byte("x") },
		"unsupported-family": func(f map[string][]byte) {
			f[ManifestName] = bytes.ReplaceAll(f[ManifestName], []byte(family), []byte("komari-1.2.7"))
		},
		"unsupported-format": func(f map[string][]byte) {
			f[ManifestName] = bytes.ReplaceAll(f[ManifestName], []byte(`"format":1`), []byte(`"format":2`))
		},
		"invalid-sqlite-with-matching-hash": func(f map[string][]byte) {
			f["komari.db"] = []byte("not a database")
			p := filepath.Join(t.TempDir(), "invalid.db")
			put(t, p, f["komari.db"])
			e, _ := fileDigest(p)
			var m manifest
			if err := json.Unmarshal(f[ManifestName], &m); err != nil {
				t.Fatal(err)
			}
			m.Files["komari.db"] = e
			f[ManifestName], _ = json.Marshal(m)
		},
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			data := t.TempDir()
			put(t, filepath.Join(data, "komari.db"), []byte("original"))
			put(t, filepath.Join(data, PendingName), editArchive(t, good, edit))
			if err := ApplyPending(data, filepath.Join(data, "komari.db")); err == nil {
				t.Fatal("unsafe backup accepted")
			}
			assertContents(t, filepath.Join(data, "komari.db"), "original")
			if _, err := os.Stat(filepath.Join(data, PendingName)); err != nil {
				t.Fatal("pending archive lost")
			}
		})
	}
}

func TestRestoreRollsBackMoveFailures(t *testing.T) {
	// Fail each move of two original files and three replacement entries.
	for failAt := 1; failAt <= 5; failAt++ {
		t.Run(string(rune('0'+failAt)), func(t *testing.T) {
			data := t.TempDir()
			put(t, filepath.Join(data, "komari.db"), []byte("original"))
			put(t, filepath.Join(data, "settings.txt"), []byte("original settings"))
			put(t, filepath.Join(data, PendingName), fixture(t))
			calls := 0
			err := applyPending(data, filepath.Join(data, "komari.db"), func(a, b string) error {
				calls++
				if calls == failAt {
					return errors.New("simulated filesystem failure")
				}
				return os.Rename(a, b)
			})
			if err == nil || !strings.Contains(err.Error(), "original data restored") {
				t.Fatalf("unexpected result %v", err)
			}
			assertContents(t, filepath.Join(data, "komari.db"), "original")
			assertContents(t, filepath.Join(data, "settings.txt"), "original settings")
			if _, err := os.Stat(filepath.Join(data, JournalName)); err != nil {
				t.Fatal("rollback must retain the journal to stop automatic retries")
			}
			before, _ := os.ReadDir(filepath.Join(data, HistoryName))
			for restart := 0; restart < 3; restart++ {
				if err := ApplyPending(data, filepath.Join(data, "komari.db")); err == nil {
					t.Fatal("automatic restart retried a failed restore")
				}
			}
			after, _ := os.ReadDir(filepath.Join(data, HistoryName))
			if len(before) != len(after) {
				t.Fatal("automatic restart created more restore directories")
			}
			assertContents(t, filepath.Join(data, "komari.db"), "original")
		})
	}
}

func TestInterruptedRestoreAndCustomPathStopStartup(t *testing.T) {
	data := t.TempDir()
	put(t, filepath.Join(data, JournalName), []byte("interrupted"))
	if err := ApplyPending(data, filepath.Join(data, "komari.db")); err == nil {
		t.Fatal("interrupted restore allowed startup")
	}
	os.Remove(filepath.Join(data, JournalName))
	put(t, filepath.Join(data, PendingName), fixture(t))
	if err := ApplyPending(data, filepath.Join(data, "custom.db")); err == nil {
		t.Fatal("custom database target accepted")
	}
}

func TestTruncatedZIPDoesNotTouchLiveData(t *testing.T) {
	good := fixture(t)
	data := t.TempDir()
	put(t, filepath.Join(data, "komari.db"), []byte("original"))
	put(t, filepath.Join(data, PendingName), good[:len(good)/2])
	if err := ApplyPending(data, filepath.Join(data, "komari.db")); err == nil {
		t.Fatal("truncated archive accepted")
	}
	assertContents(t, filepath.Join(data, "komari.db"), "original")
}
