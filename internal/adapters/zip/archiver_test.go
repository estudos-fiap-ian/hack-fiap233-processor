package zip

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestArchive_CreatesValidZIP(t *testing.T) {
	dir := t.TempDir()

	// Create source files.
	files := createTestFiles(t, dir, map[string]string{
		"frame_0001.png": "png-content-1",
		"frame_0002.png": "png-content-2",
	})

	zipPath := filepath.Join(dir, "output.zip")
	a := New()
	if err := a.Archive(files, zipPath); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	if _, err := os.Stat(zipPath); err != nil {
		t.Fatalf("ZIP file not created: %v", err)
	}
}

func TestArchive_ContainsAllFiles(t *testing.T) {
	dir := t.TempDir()
	files := createTestFiles(t, dir, map[string]string{
		"frame_0001.png": "data1",
		"frame_0002.png": "data2",
		"frame_0003.png": "data3",
	})

	zipPath := filepath.Join(dir, "output.zip")
	if err := New().Archive(files, zipPath); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	names := readZIPEntries(t, zipPath)
	if len(names) != 3 {
		t.Fatalf("expected 3 entries in ZIP, got %d: %v", len(names), names)
	}
}

func TestArchive_FileContentsArePreserved(t *testing.T) {
	dir := t.TempDir()
	files := createTestFiles(t, dir, map[string]string{
		"frame_0001.png": "exact-content-here",
	})

	zipPath := filepath.Join(dir, "output.zip")
	if err := New().Archive(files, zipPath); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("failed to open ZIP: %v", err)
	}
	defer r.Close()

	if len(r.File) != 1 {
		t.Fatalf("expected 1 file in ZIP, got %d", len(r.File))
	}

	rc, err := r.File[0].Open()
	if err != nil {
		t.Fatalf("failed to open ZIP entry: %v", err)
	}
	defer rc.Close()

	buf := make([]byte, 100)
	n, _ := rc.Read(buf)
	if string(buf[:n]) != "exact-content-here" {
		t.Errorf("content mismatch: got %q", string(buf[:n]))
	}
}

func TestArchive_EmptyFileList(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "empty.zip")

	if err := New().Archive([]string{}, zipPath); err != nil {
		t.Fatalf("Archive with empty list failed: %v", err)
	}

	names := readZIPEntries(t, zipPath)
	if len(names) != 0 {
		t.Errorf("expected empty ZIP, got entries: %v", names)
	}
}

func TestArchive_UsesOnlyFilename(t *testing.T) {
	dir := t.TempDir()
	files := createTestFiles(t, dir, map[string]string{
		"frame_0001.png": "data",
	})

	zipPath := filepath.Join(dir, "output.zip")
	if err := New().Archive(files, zipPath); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	names := readZIPEntries(t, zipPath)
	if len(names) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(names))
	}
	// ZIP entry must use base filename, not full path.
	if names[0] != "frame_0001.png" {
		t.Errorf("expected entry name 'frame_0001.png', got %q", names[0])
	}
}

func TestArchive_UsesDeflateCompression(t *testing.T) {
	dir := t.TempDir()
	files := createTestFiles(t, dir, map[string]string{
		"frame_0001.png": "compressible-data-aaaaaaaaaaaaaaaaaaaaa",
	})

	zipPath := filepath.Join(dir, "output.zip")
	if err := New().Archive(files, zipPath); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("failed to open ZIP: %v", err)
	}
	defer r.Close()

	if r.File[0].Method != zip.Deflate {
		t.Errorf("expected Deflate compression, got method %d", r.File[0].Method)
	}
}

func TestArchive_InvalidSourceFile_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "output.zip")

	err := New().Archive([]string{"/nonexistent/path/frame.png"}, zipPath)
	if err == nil {
		t.Fatal("expected error for non-existent source file")
	}
}

func TestArchive_InvalidDestinationPath_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	files := createTestFiles(t, dir, map[string]string{"f.png": "d"})

	err := New().Archive(files, "/nonexistent/dir/output.zip")
	if err == nil {
		t.Fatal("expected error for invalid destination path")
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func createTestFiles(t *testing.T, dir string, contents map[string]string) []string {
	t.Helper()
	var paths []string
	for name, content := range contents {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create test file %s: %v", name, err)
		}
		paths = append(paths, path)
	}
	return paths
}

func readZIPEntries(t *testing.T, zipPath string) []string {
	t.Helper()
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("failed to open ZIP: %v", err)
	}
	defer r.Close()
	var names []string
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	return names
}
