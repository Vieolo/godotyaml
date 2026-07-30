package godotyaml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingFileIsNotExist(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "go.yaml"))
	if err == nil {
		t.Fatal("Load of a missing file returned no error")
	}
	if !os.IsNotExist(err) {
		t.Errorf("error should satisfy os.IsNotExist so callers can skip a stat, got %v", err)
	}
}

// TestSavePreservesExistingFileMode pins behaviour that looks like a bug and is
// not: Save names 0644, but a file that already exists keeps its own mode. A
// go.yaml deliberately kept at 0600 must not be widened by a tool writing to it.
func TestSavePreservesExistingFileMode(t *testing.T) {
	for _, mode := range []os.FileMode{0o600, 0o640, 0o604} {
		t.Run(mode.String(), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "go.yaml")
			if err := os.WriteFile(path, []byte("name: x\n"), mode); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, mode); err != nil { // defeat the umask
				t.Fatal(err)
			}

			doc, err := Load(path)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if err := doc.SetExternalConfig("tool", map[string]any{"a": 1}); err != nil {
				t.Fatalf("SetExternalConfig: %v", err)
			}
			if err := doc.Save(path); err != nil {
				t.Fatalf("Save: %v", err)
			}

			fi, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := fi.Mode().Perm(); got != mode {
				t.Errorf("mode after Save = %v, want %v (Save must not change an existing file's permissions)", got, mode)
			}
		})
	}
}

// A file Save creates should land on the same mode os.WriteFile would give it,
// umask included.
func TestSaveNewFileMatchesWriteFileMode(t *testing.T) {
	dir := t.TempDir()

	ref := filepath.Join(dir, "ref.yaml")
	if err := os.WriteFile(ref, []byte("name: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	refInfo, err := os.Stat(ref)
	if err != nil {
		t.Fatal(err)
	}

	doc, err := Parse(strings.NewReader("name: x\n"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "go.yaml")
	if err := doc.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := fi.Mode().Perm(), refInfo.Mode().Perm(); got != want {
		t.Errorf("new file mode = %v, want %v (same as os.WriteFile)", got, want)
	}
}

// Save writes through a temporary file; it must not leave one behind.
func TestSaveLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.yaml")

	doc, err := Parse(strings.NewReader("name: x\n"))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 { // once creating, once replacing
		if err := doc.Save(path); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "go.yaml" {
			t.Errorf("Save left %q behind", e.Name())
		}
	}
}

// os.Rename replaces a symlink rather than writing through it, so Save resolves
// the link first and the link must survive.
func TestSaveFollowsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.yaml")
	link := filepath.Join(dir, "go.yaml")

	if err := os.WriteFile(target, []byte("name: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real.yaml", link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	doc, err := Load(link)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := doc.SetExternalConfig("tool", map[string]any{"a": 1}); err != nil {
		t.Fatal(err)
	}
	if err := doc.Save(link); err != nil {
		t.Fatalf("Save: %v", err)
	}

	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("Save replaced the symlink with a regular file")
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "tool:") {
		t.Errorf("symlink target not updated:\n%s", content)
	}
	if tfi, err := os.Stat(target); err == nil && tfi.Mode().Perm() != 0o600 {
		t.Errorf("symlink target mode = %v, want 0600", tfi.Mode().Perm())
	}
}

// A failed Save must leave the previous file intact rather than a truncated one.
func TestSaveFailureLeavesOriginalIntact(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "go.yaml")
	const original = "name: x\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	doc, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := doc.SetExternalConfig("tool", map[string]any{"a": 1}); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(dir, 0o500); err != nil { // no new files in this directory
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if err := doc.Save(path); err == nil {
		t.Fatal("Save into a read-only directory should fail")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != original {
		t.Errorf("failed Save damaged the file:\n%s", content)
	}
}

// Round trip through disk: a save/load cycle must not disturb the document.
func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.yaml")
	if err := os.WriteFile(path, []byte(sectionDoc), 0o644); err != nil {
		t.Fatal(err)
	}

	doc, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := doc.MergeExternalConfig("contour", map[string]any{"bootstrap": []string{"go"}}); err != nil {
		t.Fatal(err)
	}
	if err := doc.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := string(content)
	mustContain(t, out,
		"# a comment before external",
		"# a comment someone wrote by hand inside contour's section",
		"note: keep-me",
		"build: \"go build ./...\"",
	)
}
