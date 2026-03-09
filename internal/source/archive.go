package source

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ArchiveResult struct {
	SourcePath   string
	ArchivePath  string
	ChecksumPath string
	Digest       string
}

func CreateCookbookArchive(sourcePath, archivePath, checksumPath string) (*ArchiveResult, error) {
	if filepath.Base(filepath.Clean(sourcePath)) != "cookbooks" {
		return nil, fmt.Errorf("a cookbook directory could not be found at %s", sourcePath)
	}

	resolvedSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("resolve source path: %w", err)
	}
	info, err := os.Stat(resolvedSource)
	if err != nil {
		return nil, fmt.Errorf("a cookbook directory could not be found at %s", sourcePath)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("a cookbook directory could not be found at %s", sourcePath)
	}

	resolvedArchive, err := filepath.Abs(archivePath)
	if err != nil {
		return nil, fmt.Errorf("resolve archive path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(resolvedArchive), 0o755); err != nil {
		return nil, fmt.Errorf("create archive directory: %w", err)
	}

	artifact, err := os.Create(resolvedArchive)
	if err != nil {
		return nil, fmt.Errorf("create archive file: %w", err)
	}
	defer artifact.Close()

	hasher := sha256.New()
	tee := io.MultiWriter(artifact, hasher)
	gz := gzip.NewWriter(tee)
	tw := tar.NewWriter(gz)

	if err := writeArchiveTree(tw, resolvedSource); err != nil {
		_ = tw.Close()
		_ = gz.Close()
		return nil, err
	}
	if err := tw.Close(); err != nil {
		_ = gz.Close()
		return nil, fmt.Errorf("close tar writer: %w", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("close gzip writer: %w", err)
	}

	digest := hex.EncodeToString(hasher.Sum(nil))

	resolvedChecksum := ""
	if checksumPath != "" {
		resolvedChecksum, err = filepath.Abs(checksumPath)
		if err != nil {
			return nil, fmt.Errorf("resolve checksum path: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(resolvedChecksum), 0o755); err != nil {
			return nil, fmt.Errorf("create checksum directory: %w", err)
		}
		content := fmt.Sprintf("%s  %s\n", digest, filepath.Base(resolvedArchive))
		if err := os.WriteFile(resolvedChecksum, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("write checksum file: %w", err)
		}
	}

	return &ArchiveResult{SourcePath: resolvedSource, ArchivePath: resolvedArchive, ChecksumPath: resolvedChecksum, Digest: digest}, nil
}

func writeArchiveTree(tw *tar.Writer, sourceRoot string) error {
	entries := make([]string, 0)
	if err := filepath.WalkDir(sourceRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		entries = append(entries, path)
		return nil
	}); err != nil {
		return fmt.Errorf("walk source tree: %w", err)
	}
	sort.Strings(entries)

	for _, entry := range entries {
		rel, err := filepath.Rel(sourceRoot, entry)
		if err != nil {
			return fmt.Errorf("compute relative archive path: %w", err)
		}
		archiveName := "cookbooks"
		if rel != "." {
			archiveName = filepath.ToSlash(filepath.Join("cookbooks", rel))
		}

		info, err := os.Lstat(entry)
		if err != nil {
			return fmt.Errorf("read entry metadata %q: %w", entry, err)
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("build tar header %q: %w", entry, err)
		}
		hdr.Name = archiveName
		if info.IsDir() && !strings.HasSuffix(hdr.Name, "/") {
			hdr.Name += "/"
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("write tar header %q: %w", hdr.Name, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}

		file, err := os.Open(entry)
		if err != nil {
			return fmt.Errorf("open source file %q: %w", entry, err)
		}
		if _, err := io.Copy(tw, file); err != nil {
			_ = file.Close()
			return fmt.Errorf("write tar body %q: %w", hdr.Name, err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close source file %q: %w", entry, err)
		}
	}
	return nil
}
