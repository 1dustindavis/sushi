package source

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateCookbookArchive(t *testing.T) {
	sourceRoot := filepath.Join(t.TempDir(), "cookbooks")
	if err := os.MkdirAll(filepath.Join(sourceRoot, "base", "recipes"), 0o755); err != nil {
		t.Fatalf("mkdir source tree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "base", "recipes", "default.rb"), []byte("file '/tmp/ok'\n"), 0o644); err != nil {
		t.Fatalf("write cookbook file: %v", err)
	}

	artifact := filepath.Join(t.TempDir(), "artifact.tar.gz")
	checksum := artifact + ".sha256"
	result, err := CreateCookbookArchive(sourceRoot, artifact, checksum)
	if err != nil {
		t.Fatalf("CreateCookbookArchive returned error: %v", err)
	}
	if result.Digest == "" {
		t.Fatal("expected digest")
	}

	checksumBytes, err := os.ReadFile(checksum)
	if err != nil {
		t.Fatalf("read checksum: %v", err)
	}
	if !strings.Contains(string(checksumBytes), result.Digest) {
		t.Fatalf("checksum file missing digest, got %q", checksumBytes)
	}

	file, err := os.Open(artifact)
	if err != nil {
		t.Fatalf("open artifact: %v", err)
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("open gzip: %v", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	seen := false
	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("read tar: %v", err)
		}
		if hdr.Name == "cookbooks/base/recipes/default.rb" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("expected cookbook file in artifact")
	}
}

func TestCreateCookbookArchiveRejectsNonCookbooksDirectory(t *testing.T) {
	sourceRoot := filepath.Join(t.TempDir(), "not-cookbooks")
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		t.Fatalf("mkdir source tree: %v", err)
	}

	_, err := CreateCookbookArchive(sourceRoot, filepath.Join(t.TempDir(), "artifact.tar.gz"), "")
	if err == nil {
		t.Fatal("expected an error")
	}

	want := "a cookbook directory could not be found at " + sourceRoot
	if err.Error() != want {
		t.Fatalf("unexpected error\nwant: %q\ngot:  %q", want, err.Error())
	}
}
