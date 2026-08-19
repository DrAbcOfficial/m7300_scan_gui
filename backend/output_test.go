package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanBasePath(t *testing.T) {
	cases := []struct {
		format string
		want   string
	}{
		{"png", "/out/dox/dox"},
		{"jpg", "/out/dox/dox"},
		{"pdf-page", "/out/dox/dox"},
		{"pdf", "/out/dox"},
	}
	for _, c := range cases {
		s := Settings{OutputDir: "/out", OutputBase: "dox", Format: c.format}
		if got := scanBasePath(s); got != c.want {
			t.Errorf("scanBasePath(%s) = %q, want %q", c.format, got, c.want)
		}
	}
}

func TestScanBasePathStripsFormatSuffix(t *testing.T) {
	s := Settings{OutputDir: "/out", OutputBase: "dox.pdf", Format: "pdf"}
	if got, want := scanBasePath(s), "/out/dox"; got != want {
		t.Fatalf("scanBasePath() = %q, want %q", got, want)
	}
}

func TestBuildScanArgsOutputSubfolder(t *testing.T) {
	s := Settings{OutputDir: "/out", OutputBase: "dox", Format: "jpg"}
	args := BuildScanArgs(s)
	found := false
	for i, a := range args {
		if a == "-o" && i+1 < len(args) {
			found = true
			if !strings.HasPrefix(args[i+1], "/out/dox/") {
				t.Errorf("-o = %q, want subfolder under /out/dox/", args[i+1])
			}
		}
	}
	if !found {
		t.Error("BuildScanArgs did not emit -o")
	}
}

func TestCollectOutputFilesScattered(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "dox")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"dox_0001.png", "dox_0002.png"} {
		if err := os.WriteFile(filepath.Join(sub, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// a stray file at the root must not be picked up
	if err := os.WriteFile(filepath.Join(dir, "dox_0003.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Settings{OutputDir: dir, OutputBase: "dox", Format: "png"}
	files := collectOutputFiles(s)
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2: %v", len(files), files)
	}
	for _, f := range files {
		if filepath.Dir(f) != sub {
			t.Errorf("file %q not inside subfolder %q", f, sub)
		}
	}
}

func TestCollectOutputFilesAddsMissingExtension(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "dox")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	withoutExt := filepath.Join(sub, "dox_0001")
	if err := os.WriteFile(withoutExt, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Settings{OutputDir: dir, OutputBase: "dox", Format: "png"}
	files := collectOutputFiles(s)
	if len(files) != 1 || files[0] != withoutExt+".png" {
		t.Fatalf("got %v, want [%s]", files, withoutExt+".png")
	}
	if _, err := os.Stat(withoutExt); !os.IsNotExist(err) {
		t.Fatalf("extensionless file still exists: %v", err)
	}
}

func TestCollectMergedPDFAddsMissingExtension(t *testing.T) {
	dir := t.TempDir()
	withoutExt := filepath.Join(dir, "dox")
	if err := os.WriteFile(withoutExt, []byte("pdf"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Settings{OutputDir: dir, OutputBase: "dox", Format: "pdf"}
	files := collectOutputFiles(s)
	if len(files) != 1 || files[0] != withoutExt+".pdf" {
		t.Fatalf("got %v, want [%s]", files, withoutExt+".pdf")
	}
}

func TestCollectOutputFilesReplacesExistingTargetWithoutDuplicates(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "dox")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	withoutExt := filepath.Join(sub, "dox_0001")
	target := withoutExt + ".png"
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(withoutExt, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Settings{OutputDir: dir, OutputBase: "dox", Format: "png"}
	files := collectOutputFiles(s)
	if len(files) != 1 || files[0] != target {
		t.Fatalf("got %v, want [%s]", files, target)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("target contains %q, want new scan data", data)
	}
}
