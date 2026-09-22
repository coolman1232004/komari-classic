package backup

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultDatabase prevents an archive from overwriting a different data store
// while the process continues using a custom database path.
func DefaultDatabase(dataDir, databaseFile string) bool {
	if databaseFile == "" {
		databaseFile = "./data/komari.db"
	}
	a, e1 := filepath.Abs(databaseFile)
	b, e2 := filepath.Abs(filepath.Join(dataDir, "komari.db"))
	return e1 == nil && e2 == nil && a == b
}

// ApplyPending runs before any database connection or background worker opens.
// Old files are retained on the same persistent volume. A process interruption
// leaves a journal that stops startup for operator recovery instead of opening
// an incomplete database. This is not a multi-file atomic filesystem operation.
func ApplyPending(dataDir, databaseFile string) error {
	return applyPending(dataDir, databaseFile, os.Rename)
}

func applyPending(dataDir, databaseFile string, rename func(string, string) error) error {
	journal := filepath.Join(dataDir, JournalName)
	if _, err := os.Lstat(journal); err == nil {
		return fmt.Errorf("interrupted restore: preserve the data volume and recover using %s", journal)
	} else if !os.IsNotExist(err) {
		return err
	}
	pending := filepath.Join(dataDir, PendingName)
	info, err := os.Lstat(pending)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("pending backup must be a regular file")
	}
	if !DefaultDatabase(dataDir, databaseFile) {
		return fmt.Errorf("restore requires the default data/komari.db database location")
	}
	history := filepath.Join(dataDir, HistoryName)
	if info, err := os.Lstat(history); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("invalid restore history directory")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(history, 0700); err != nil {
		return err
	}
	txn, err := os.MkdirTemp(history, "restore-")
	if err != nil {
		return err
	}
	stage, previous := filepath.Join(txn, "new"), filepath.Join(txn, "previous")
	if err := os.Mkdir(stage, 0700); err != nil {
		return err
	}
	if err := Stage(pending, stage); err != nil {
		os.RemoveAll(txn)
		return fmt.Errorf("backup rejected; original data unchanged: %w", err)
	}
	if err := os.Mkdir(previous, 0700); err != nil {
		return err
	}
	oldFiles, err := os.ReadDir(dataDir)
	if err != nil {
		return err
	}
	newFiles, err := os.ReadDir(stage)
	if err != nil {
		return err
	}
	var oldNames, newNames []string
	for _, f := range oldFiles {
		if f.Name() != HistoryName && f.Name() != PendingName {
			oldNames = append(oldNames, f.Name())
		}
	}
	for _, f := range newFiles {
		newNames = append(newNames, f.Name())
	}
	// Write and sync recovery instructions before the first live file moves.
	f, err := os.OpenFile(journal, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	err = json.NewEncoder(f).Encode(struct {
		Transaction string   `json:"transaction"`
		OldFiles    []string `json:"old_files"`
		NewFiles    []string `json:"new_files"`
	}{txn, oldNames, newNames})
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	var movedOld, installed []string
	rollback := func(cause error) error {
		var failures []error
		for i := len(installed) - 1; i >= 0; i-- {
			name := installed[i]
			if err := os.Rename(filepath.Join(dataDir, name), filepath.Join(stage, name)); err != nil {
				failures = append(failures, err)
			}
		}
		for i := len(movedOld) - 1; i >= 0; i-- {
			name := movedOld[i]
			if err := os.Rename(filepath.Join(previous, name), filepath.Join(dataDir, name)); err != nil {
				failures = append(failures, err)
			}
		}
		if len(failures) == 0 {
			if err := os.Remove(journal); err != nil {
				failures = append(failures, err)
			}
		}
		if len(failures) > 0 {
			return fmt.Errorf("restore failed (%v); manual recovery required using %s: %w", cause, journal, errors.Join(failures...))
		}
		return fmt.Errorf("restore failed; original data restored, pending archive retained: %w", cause)
	}
	for _, name := range oldNames {
		if err := rename(filepath.Join(dataDir, name), filepath.Join(previous, name)); err != nil {
			return rollback(err)
		}
		movedOld = append(movedOld, name)
	}
	for _, name := range newNames {
		if err := rename(filepath.Join(stage, name), filepath.Join(dataDir, name)); err != nil {
			return rollback(err)
		}
		installed = append(installed, name)
	}
	// Preserve the exact source archive alongside the previous installation.
	if err := os.Rename(pending, filepath.Join(txn, "source.zip")); err != nil {
		return rollback(err)
	}
	if err := os.Remove(journal); err != nil {
		return fmt.Errorf("restore installed; startup stopped until journal recovery: %w", err)
	}
	return nil
}
