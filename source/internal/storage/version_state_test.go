package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validVersionMetadata(name string) Metadata {
	return Metadata{
		Name:     name,
		Size:     5,
		SHA256:   strings.Repeat("a", 64),
		Version:  1,
		WriterID: "node-a",
	}
}

func TestValidateVersionedMetadata(t *testing.T) {
	valid := validVersionMetadata("report.txt")

	if err := validateVersionedMetadata(valid, 1024); err != nil {
		t.Fatalf("valid metadata was rejected: %v", err)
	}

	tests := []struct {
		name     string
		metadata Metadata
	}{
		{"missing version", func() Metadata {
			m := valid
			m.Version = 0
			return m
		}()},
		{"missing writer", func() Metadata {
			m := valid
			m.WriterID = ""
			return m
		}()},
		{"invalid digest", func() Metadata {
			m := valid
			m.SHA256 = strings.Repeat("z", 64)
			return m
		}()},
		{"invalid digest length", func() Metadata {
			m := valid
			m.SHA256 = "abcd"
			return m
		}()},
		{"oversized file", func() Metadata {
			m := valid
			m.Size = 2048
			return m
		}()},
		{"invalid filename", func() Metadata {
			m := valid
			m.Name = "../report.txt"
			return m
		}()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateVersionedMetadata(test.metadata, 1024); err == nil {
				t.Fatal("invalid metadata was accepted")
			}
		})
	}
}

func TestValidateVersionManifestRejectsDuplicates(t *testing.T) {
	manifest := versionManifest{
		Format: versionStateFormat,
		Clock:  1,
		Files: []Metadata{
			validVersionMetadata("report.txt"),
			validVersionMetadata("report.txt"),
		},
	}

	if _, err := validateVersionManifest(manifest, 1024); err == nil {
		t.Fatal("manifest with duplicate filenames was accepted")
	}
}

func TestValidateVersionManifestRejectsClockBelowFileVersion(t *testing.T) {
	metadata := validVersionMetadata("report.txt")
	metadata.Version = 2

	manifest := versionManifest{
		Format: versionStateFormat,
		Clock:  1,
		Files:  []Metadata{metadata},
	}

	if _, err := validateVersionManifest(manifest, 1024); err == nil {
		t.Fatal("manifest clock below a file version was accepted")
	}
}

func TestSortedVersionMetadata(t *testing.T) {
	files := map[string]Metadata{
		"z.txt": validVersionMetadata("z.txt"),
		"a.txt": validVersionMetadata("a.txt"),
		"m.txt": validVersionMetadata("m.txt"),
	}

	got := sortedVersionMetadata(files)
	if len(got) != 3 {
		t.Fatalf("expected 3 records, got %d", len(got))
	}

	want := []string{"a.txt", "m.txt", "z.txt"}
	for i := range want {
		if got[i].Name != want[i] {
			t.Fatalf("record %d: got %q, want %q", i, got[i].Name, want[i])
		}
	}
}

func TestPrepareVersionStateDir(t *testing.T) {
	root := t.TempDir()

	dir, err := prepareVersionStateDir(root)
	if err != nil {
		t.Fatalf("prepareVersionStateDir failed: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("state directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("state path is not a directory")
	}
	if info.Mode().Perm() != 0700 {
		t.Fatalf("state directory permissions = %o, want 700", info.Mode().Perm())
	}
}

func TestPrepareVersionStateDirRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := t.TempDir()

	if err := os.Symlink(target, filepath.Join(root, versionStateDirName)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	if _, err := prepareVersionStateDir(root); err == nil {
		t.Fatal("symlinked state directory was accepted")
	}
}

func TestVersionJSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, versionManifestName)

	want := versionManifest{
		Format: versionStateFormat,
		Clock:  1,
		Files:  []Metadata{validVersionMetadata("report.txt")},
	}

	if err := writeVersionJSON(path, want); err != nil {
		t.Fatalf("writeVersionJSON failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("state file was not created: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("state file permissions = %o, want 600", info.Mode().Perm())
	}

	var got versionManifest
	if err := readVersionJSON(path, &got); err != nil {
		t.Fatalf("readVersionJSON failed: %v", err)
	}

	if got.Format != want.Format || got.Clock != want.Clock ||
		len(got.Files) != 1 || got.Files[0] != want.Files[0] {
		t.Fatalf("round trip mismatch: got %#v, want %#v", got, want)
	}
}

func TestReadVersionJSONRejectsInvalidFiles(t *testing.T) {
	t.Run("unknown field", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), versionManifestName)
		if err := os.WriteFile(path, []byte(`{"format":1,"clock":0,"files":[],"unexpected":true}`), 0600); err != nil {
			t.Fatal(err)
		}

		var got versionManifest
		if err := readVersionJSON(path, &got); err == nil {
			t.Fatal("unknown JSON field was accepted")
		}
	})

	t.Run("trailing JSON", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), versionManifestName)
		if err := os.WriteFile(path, []byte(`{"format":1,"clock":0,"files":[]} {}`), 0600); err != nil {
			t.Fatal(err)
		}

		var got versionManifest
		if err := readVersionJSON(path, &got); err == nil {
			t.Fatal("trailing JSON data was accepted")
		}
	})

	t.Run("symlink", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target.json")
		path := filepath.Join(dir, versionManifestName)

		if err := os.WriteFile(target, []byte(`{"format":1,"clock":0,"files":[]}`), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}

		var got versionManifest
		if err := readVersionJSON(path, &got); err == nil {
			t.Fatal("symlinked state file was accepted")
		}
	})
}

func TestVersionJSONEnforcesSizeLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, versionManifestName)

	oversized := strings.Repeat(" ", maxVersionStateBytes+1)
	if err := os.WriteFile(path, []byte(oversized), 0600); err != nil {
		t.Fatal(err)
	}

	var got versionManifest
	if err := readVersionJSON(path, &got); err == nil {
		t.Fatal("oversized state file was accepted")
	}

	if err := writeVersionJSON(path, strings.Repeat("x", maxVersionStateBytes+1)); err == nil {
		t.Fatal("oversized state data was written")
	}
}

func TestWriteVersionJSONRejectsSymlinkDestination(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	path := filepath.Join(dir, versionManifestName)

	if err := os.WriteFile(target, []byte("keep this file unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	value := versionManifest{Format: versionStateFormat, Clock: 0}
	if err := writeVersionJSON(path, value); err == nil {
		t.Fatal("writer accepted a symlink destination")
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "keep this file unchanged" {
		t.Fatal("target file was unexpectedly changed")
	}
}

func TestVersionManifestSaveAndLoad(t *testing.T) {
	dir, err := prepareVersionStateDir(t.TempDir())
	if err != nil {
		t.Fatalf("prepareVersionStateDir failed: %v", err)
	}

	files := map[string]Metadata{
		"report.txt": validVersionMetadata("report.txt"),
	}
	files["report.txt"] = func() Metadata {
		m := files["report.txt"]
		m.Version = 7
		return m
	}()

	if err := saveVersionManifest(dir, 7, files, 1024); err != nil {
		t.Fatalf("saveVersionManifest failed: %v", err)
	}

	clock, got, err := loadVersionManifest(dir, 1024)
	if err != nil {
		t.Fatalf("loadVersionManifest failed: %v", err)
	}

	if clock != 7 {
		t.Fatalf("loaded clock = %d, want 7", clock)
	}
	if len(got) != 1 || got["report.txt"] != files["report.txt"] {
		t.Fatalf("loaded files = %#v, want %#v", got, files)
	}
}

func TestVersionJournalSaveAndLoad(t *testing.T) {
	dir, err := prepareVersionStateDir(t.TempDir())
	if err != nil {
		t.Fatalf("prepareVersionStateDir failed: %v", err)
	}

	want := versionJournal{
		Format:   versionStateFormat,
		TempName: versionStagePrefix + "abc123",
		Metadata: validVersionMetadata("report.txt"),
	}

	if err := saveVersionJournal(dir, want, 1024); err != nil {
		t.Fatalf("saveVersionJournal failed: %v", err)
	}

	got, err := loadVersionJournal(dir, 1024)
	if err != nil {
		t.Fatalf("loadVersionJournal failed: %v", err)
	}
	if got != want {
		t.Fatalf("loaded journal = %#v, want %#v", got, want)
	}
}

func TestValidateVersionJournalRejectsUnsafeNames(t *testing.T) {
	valid := versionJournal{
		Format:   versionStateFormat,
		TempName: versionStagePrefix + "abc123",
		Metadata: validVersionMetadata("report.txt"),
	}

	tests := []struct {
		name     string
		tempName string
	}{
		{"empty suffix", versionStagePrefix},
		{"path traversal", versionStagePrefix + "../outside"},
		{"forward slash", versionStagePrefix + "abc/def"},
		{"backslash", versionStagePrefix + `abc\def`},
		{"invalid character", versionStagePrefix + "abc.def"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			journal := valid
			journal.TempName = test.tempName

			if err := validateVersionJournal(journal, 1024); err == nil {
				t.Fatal("unsafe journal filename was accepted")
			}
		})
	}
}

func TestLoadVersionJournalRejectsUnsafeName(t *testing.T) {
	dir, err := prepareVersionStateDir(t.TempDir())
	if err != nil {
		t.Fatalf("prepareVersionStateDir failed: %v", err)
	}

	journal := versionJournal{
		Format:   versionStateFormat,
		TempName: versionStagePrefix + "../outside",
		Metadata: validVersionMetadata("report.txt"),
	}
	if err := writeVersionJSON(filepath.Join(dir, versionJournalName), journal); err != nil {
		t.Fatalf("writeVersionJSON failed: %v", err)
	}

	if _, err := loadVersionJournal(dir, 1024); err == nil {
		t.Fatal("unsafe journal filename was accepted when loading")
	}
}

func TestVersionMarkerSaveAndLoad(t *testing.T) {
	dir, err := prepareVersionStateDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if err := saveVersionMarker(dir, "node-a"); err != nil {
		t.Fatalf("saveVersionMarker failed: %v", err)
	}
	if err := loadVersionMarker(dir, "node-a"); err != nil {
		t.Fatalf("loadVersionMarker failed: %v", err)
	}
}

func TestVersionMarkerRejectsDifferentNode(t *testing.T) {
	dir, err := prepareVersionStateDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if err := saveVersionMarker(dir, "node-a"); err != nil {
		t.Fatal(err)
	}
	if err := loadVersionMarker(dir, "node-b"); err == nil {
		t.Fatal("marker belonging to another node was accepted")
	}
}

func TestVersionMarkerRejectsInvalidNode(t *testing.T) {
	dir, err := prepareVersionStateDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if err := saveVersionMarker(dir, "../node-a"); err == nil {
		t.Fatal("invalid node ID was accepted")
	}
}
