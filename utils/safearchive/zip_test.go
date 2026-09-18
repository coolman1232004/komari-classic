package safearchive

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func archive(t *testing.T, name string, mode os.FileMode, size uint64) []*zip.File {
	t.Helper()
	return []*zip.File{{FileHeader: zip.FileHeader{Name: name, UncompressedSize64: size, ExternalAttrs: uint32(mode)}}}
}
func TestRejectUnsafeArchives(t *testing.T) {
	for _, name := range []string{"../outside", "/absolute", "x/../../outside", "x\\..\\outside", "C:/outside"} {
		if Validate(archive(t, name, 0, 0)) == nil {
			t.Errorf("accepted %q", name)
		}
	}
	if Validate(archive(t, "large", 0, MaxExpanded+1)) == nil {
		t.Fatal("bomb accepted")
	}
	f := &zip.File{FileHeader: zip.FileHeader{Name: "link"}}
	f.SetMode(os.ModeSymlink | 0777)
	if Validate([]*zip.File{f}) == nil {
		t.Fatal("symlink accepted")
	}
	if Validate(append(archive(t, "same", 0, 0), archive(t, "same", 0, 0)...)) == nil {
		t.Fatal("duplicate accepted")
	}
}
func TestExtractAndContainSymlinks(t *testing.T) {
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	f, _ := w.Create("nested/file.txt")
	f.Write([]byte("safe"))
	w.Close()
	r, err := zip.NewReader(bytes.NewReader(b.Bytes()), int64(b.Len()))
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	if err := Extract(r.File, dst); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dst, "nested/file.txt"))
	if string(data) != "safe" {
		t.Fatal("normal extraction failed")
	}
	other := t.TempDir()
	dst = t.TempDir()
	if err := os.Symlink(other, filepath.Join(dst, "nested")); err != nil {
		t.Skip("symlink privileges unavailable")
	}
	if Extract(r.File, dst) == nil {
		t.Fatal("existing symlink escaped root")
	}
	if _, err := os.Stat(filepath.Join(other, "file.txt")); !os.IsNotExist(err) {
		t.Fatal("wrote outside destination")
	}
}
