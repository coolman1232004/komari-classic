// Package backup implements the versioned Classic backup format. Its manifest
// detects corruption and unsupported formats; it does not authenticate a backup.
// Only an administrator's own trusted backups should ever be restored.
package backup

import (
	"archive/zip"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/komari-monitor/komari/utils/safearchive"
	_ "github.com/mattn/go-sqlite3"
)

const ManifestName = "komari-backup.json"
const PendingName = "backup.zip"
const HistoryName = ".komari-restore"
const JournalName = ".restore-in-progress"
const family = "komari-classic-1.2.5-fix2"

type entry struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type manifest struct {
	Format        int              `json:"format"`
	Family        string           `json:"family"`
	SourceVersion string           `json:"source_version"`
	Files         map[string]entry `json:"files"`
}

// Reserved files never belong in the payload of an exported backup.
func Reserved(name string) bool {
	name = strings.ToLower(strings.Split(filepath.ToSlash(name), "/")[0])
	return name == ManifestName || name == PendingName || name == HistoryName ||
		name == JournalName || name == "komari-backup-markup" || strings.HasPrefix(name, ".backup-upload-")
}

func fileDigest(path string) (entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return entry{}, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	return entry{n, hex.EncodeToString(h.Sum(nil))}, err
}

// Write archives a private, consistent snapshot, never the live SQLite files.
// The caller must close and check the output before serving it to a client.
func Write(w io.Writer, root, version string) error {
	m := manifest{Format: 1, Family: family, SourceVersion: version, Files: map[string]entry{}}
	z := zip.NewWriter(w)
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup cannot include symlinks")
		}
		if Reserved(rel) {
			return fmt.Errorf("reserved backup path: %s", rel)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("backup cannot include special files")
		}
		name := filepath.ToSlash(rel)
		digest, err := fileDigest(p)
		if err != nil {
			return err
		}
		m.Files[name] = digest
		out, err := z.Create(name)
		if err != nil {
			return err
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, in)
		closeErr := in.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
	if err != nil {
		z.Close()
		return err
	}
	if _, ok := m.Files["komari.db"]; !ok {
		z.Close()
		return fmt.Errorf("backup requires komari.db")
	}
	w, err = z.Create(ManifestName)
	if err == nil {
		err = json.NewEncoder(w).Encode(m)
	}
	closeErr := z.Close()
	if err != nil {
		return err
	}
	return closeErr
}

// Stage fully extracts, checks CRCs/digests and checks SQLite before any live
// data is touched. destination must be a new private directory.
func Stage(archive, destination string) error {
	z, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("invalid backup ZIP: %w", err)
	}
	defer z.Close()
	if err := safearchive.Validate(z.File); err != nil {
		return err
	}
	var mf *zip.File
	for _, f := range z.File {
		if f.Name == ManifestName {
			mf = f
			continue
		}
		if Reserved(f.Name) {
			return fmt.Errorf("reserved backup path")
		}
		name := strings.ToLower(f.Name)
		if (strings.HasSuffix(name, ".db") && name != "komari.db") ||
			strings.HasSuffix(name, "-wal") || strings.HasSuffix(name, "-shm") || strings.HasSuffix(name, "-journal") {
			return fmt.Errorf("unsupported database files in backup")
		}
	}
	if mf == nil || mf.UncompressedSize64 > 2<<20 {
		return fmt.Errorf("unsupported backup format; use a backup exported by Classic Security 3 or later (legacy and upstream backups need separate migration)")
	}
	r, err := mf.Open()
	if err != nil {
		return err
	}
	dec := json.NewDecoder(io.LimitReader(r, (2<<20)+1))
	dec.DisallowUnknownFields()
	var m manifest
	err = dec.Decode(&m)
	if err == nil {
		var extra any
		if dec.Decode(&extra) != io.EOF {
			err = fmt.Errorf("invalid manifest data")
		}
	}
	r.Close()
	if err != nil {
		return err
	}
	if m.Format != 1 || m.Family != family || m.SourceVersion == "" || len(m.SourceVersion) > 128 {
		return fmt.Errorf("unsupported backup version")
	}
	if _, ok := m.Files["komari.db"]; !ok {
		return fmt.Errorf("backup is missing komari.db")
	}
	count := 0
	for _, f := range z.File {
		if f.Name == ManifestName || f.FileInfo().IsDir() {
			continue
		}
		e, ok := m.Files[f.Name]
		if !ok || e.Size < 0 || uint64(e.Size) != f.UncompressedSize64 || len(e.SHA256) != 64 {
			return fmt.Errorf("backup manifest mismatch")
		}
		count++
	}
	if count != len(m.Files) {
		return fmt.Errorf("backup is missing files")
	}
	if err := safearchive.Extract(z.File, destination); err != nil {
		return err
	}
	for name, expected := range m.Files {
		got, err := fileDigest(filepath.Join(destination, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		if got != expected {
			return fmt.Errorf("backup checksum mismatch")
		}
	}
	if err := checkDatabase(filepath.Join(destination, "komari.db")); err != nil {
		return err
	}
	return os.Remove(filepath.Join(destination, ManifestName))
}

func openDatabase(path string) (*sql.DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	// A Windows drive letter belongs in the URI path, not its authority.
	if !strings.HasPrefix(u.Path, "/") {
		u.Path = "/" + u.Path
	}
	u.RawQuery = "mode=rw&_busy_timeout=5000&_journal_mode=DELETE"
	db, err := sql.Open("sqlite3", u.String())
	if err == nil {
		db.SetMaxOpenConns(1)
	}
	return db, err
}

func checkDatabase(path string) error {
	db, err := openDatabase(path)
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if err := db.QueryRow("PRAGMA quick_check").Scan(&result); err != nil {
		return fmt.Errorf("invalid SQLite backup: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("SQLite backup integrity check failed")
	}
	// These tables/columns are required by the Classic backup family. This is
	// an additional sanity check, not a claim of upstream migration support.
	for _, query := range []string{
		"SELECT uuid, username, passwd, two_factor FROM users LIMIT 0",
		"SELECT uuid, token FROM clients LIMIT 0",
		"SELECT key, value FROM configs LIMIT 0",
		"SELECT session, uuid FROM sessions LIMIT 0",
		"SELECT client, time FROM records LIMIT 0",
		"SELECT client, time FROM ping_records LIMIT 0",
	} {
		rows, err := db.Query(query)
		if err != nil {
			return fmt.Errorf("unsupported Classic database schema: %w", err)
		}
		rows.Close()
	}
	// Never resurrect browser sessions copied from another installation or an
	// older snapshot. Passwords, MFA secrets and agent tokens remain as backed up.
	if _, err := db.Exec("DELETE FROM sessions"); err != nil {
		return err
	}
	return db.Close()
}
