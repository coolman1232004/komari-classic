package safearchive

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const MaxExpanded uint64 = 2 << 30
const MaxFiles = 10000

// Validate all entries before touching destination files. Limits cover the
// expanded archive, not just its compressed upload size.
func Validate(files []*zip.File) error {
	if len(files) > MaxFiles {
		return fmt.Errorf("too many archive entries")
	}
	var total uint64
	seen := map[string]bool{}
	for _, f := range files {
		name := strings.TrimSuffix(f.Name, "/")
		if name == "" || strings.ContainsAny(name, "\\:\x00") || !filepath.IsLocal(name) || path.Clean(name) != name || strings.HasPrefix(name, "/") {
			return fmt.Errorf("unsafe archive path %q", f.Name)
		}
		key := strings.ToLower(name)
		if seen[key] {
			return fmt.Errorf("duplicate archive path")
		}
		seen[key] = true
		if f.Mode()&os.ModeSymlink != 0 || (!f.FileInfo().IsDir() && !f.Mode().IsRegular()) {
			return fmt.Errorf("unsupported archive entry")
		}
		if f.UncompressedSize64 > MaxExpanded-total {
			return fmt.Errorf("expanded archive exceeds 2 GiB")
		}
		total += f.UncompressedSize64
	}
	return nil
}

func Extract(files []*zip.File, destination string) error {
	if err := Validate(files); err != nil {
		return err
	}
	if err := os.MkdirAll(destination, 0755); err != nil {
		return err
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		return err
	}
	defer root.Close()
	var remaining int64 = int64(MaxExpanded)
	for _, f := range files {
		name := filepath.FromSlash(f.Name)
		if f.FileInfo().IsDir() {
			if err := root.MkdirAll(name, 0755); err != nil {
				return err
			}
			continue
		}
		if err := root.MkdirAll(filepath.Dir(name), 0755); err != nil {
			return err
		}
		in, err := f.Open()
		if err != nil {
			return err
		}
		out, err := root.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			in.Close()
			return err
		}
		n, copyErr := io.Copy(out, io.LimitReader(in, remaining+1))
		closeErr := out.Close()
		in.Close()
		remaining -= n
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if remaining < 0 {
			return fmt.Errorf("expanded archive limit exceeded")
		}
	}
	return nil
}
