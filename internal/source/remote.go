package source

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"

	"sushi/internal/config"
)

type RemoteResult struct {
	CookbookPath string
	Digest       string
	Reason       string
}

type cacheMetadata struct {
	Digest    string    `json:"digest"`
	FetchedAt time.Time `json:"fetched_at"`
	SourceURL string    `json:"source_url"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

func ResolveRemote(src config.RemoteSource) (*RemoteResult, error) {
	meta, bundlePath, _ := loadCurrentMetadata(src.CacheDir)

	if meta != nil && !shouldRefresh(*meta, src.RefreshInterval) {
		return &RemoteResult{CookbookPath: bundlePath, Digest: meta.Digest, Reason: "using cached bundle (refresh interval not elapsed)"}, nil
	}

	fetched, fetchErr := fetchAndActivateRemote(src)
	if fetchErr == nil {
		return &RemoteResult{CookbookPath: fetched.bundlePath, Digest: fetched.meta.Digest, Reason: "fetched and activated remote bundle"}, nil
	}

	if !src.AllowCachedFallback || meta == nil {
		return nil, fmt.Errorf("remote fetch failed: %v", fetchErr)
	}

	stale, age, staleErr := isCacheStale(*meta, src.MaxCacheAge)
	if staleErr != nil {
		return nil, fmt.Errorf("remote fetch failed and cache policy invalid: %v", staleErr)
	}
	if stale && src.FailIfStale {
		return nil, fmt.Errorf("remote fetch failed and cache is stale (%s old): %v", age.Round(time.Second), fetchErr)
	}

	reason := fmt.Sprintf("using cached fallback after fetch failure (%v)", fetchErr)
	if stale {
		reason = fmt.Sprintf("using stale cached fallback (%s old) after fetch failure (%v)", age.Round(time.Second), fetchErr)
	}

	return &RemoteResult{CookbookPath: bundlePath, Digest: meta.Digest, Reason: reason}, nil
}

type fetchedRemote struct {
	meta       cacheMetadata
	bundlePath string
}

func fetchAndActivateRemote(src config.RemoteSource) (*fetchedRemote, error) {
	if err := os.MkdirAll(src.CacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}

	tmpFile, err := os.CreateTemp(src.CacheDir, "bundle-*.tar")
	if err != nil {
		return nil, fmt.Errorf("create temp bundle: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	resp, err := http.Get(src.URL) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("download bundle: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("download bundle: HTTP %d", resp.StatusCode)
	}

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmpFile, h), resp.Body); err != nil {
		return nil, fmt.Errorf("write bundle: %w", err)
	}
	computedDigest := hex.EncodeToString(h.Sum(nil))

	if src.ChecksumURL != "" {
		expected, err := fetchExpectedChecksum(src.ChecksumURL)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(expected, computedDigest) {
			return nil, fmt.Errorf("checksum mismatch: expected %s got %s", expected, computedDigest)
		}
	}

	bundleRoot := filepath.Join(src.CacheDir, "bundles", computedDigest)
	cookbookPath := filepath.Join(bundleRoot, "cookbooks")
	if _, err := os.Stat(cookbookPath); err == nil {
		meta, err := writeCurrentMetadata(src.CacheDir, cacheMetadataFromDigest(src, computedDigest))
		if err != nil {
			return nil, err
		}
		return &fetchedRemote{meta: *meta, bundlePath: cookbookPath}, nil
	}

	extractRoot := bundleRoot + ".tmp"
	_ = os.RemoveAll(extractRoot)
	if err := os.MkdirAll(extractRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create temp extraction root: %w", err)
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind temp bundle: %w", err)
	}
	if err := extractBundle(tmpFile, extractRoot, src.URL); err != nil {
		_ = os.RemoveAll(extractRoot)
		return nil, err
	}
	if err := os.Rename(extractRoot, bundleRoot); err != nil {
		_ = os.RemoveAll(extractRoot)
		return nil, fmt.Errorf("activate bundle: %w", err)
	}

	meta, err := writeCurrentMetadata(src.CacheDir, cacheMetadataFromDigest(src, computedDigest))
	if err != nil {
		return nil, err
	}

	return &fetchedRemote{meta: *meta, bundlePath: filepath.Join(bundleRoot, "cookbooks")}, nil
}

func fetchExpectedChecksum(checksumURL string) (string, error) {
	resp, err := http.Get(checksumURL) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("download checksum: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("download checksum: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read checksum: %w", err)
	}
	line := strings.TrimSpace(string(body))
	if line == "" {
		return "", fmt.Errorf("checksum response was empty")
	}
	parts := strings.Fields(line)
	return strings.TrimSpace(parts[0]), nil
}

func extractBundle(file *os.File, dst string, sourceURL string) error {
	var reader io.Reader = file
	sourceURLLower := strings.ToLower(sourceURL)
	if strings.HasSuffix(sourceURLLower, ".gz") || strings.HasSuffix(sourceURLLower, ".tgz") {
		gz, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("open gzip bundle: %w", err)
		}
		defer gz.Close()
		reader = gz
	} else if strings.HasSuffix(sourceURLLower, ".zst") || strings.HasSuffix(sourceURLLower, ".rst") {
		zst, err := zstd.NewReader(file)
		if err != nil {
			return fmt.Errorf("open zstd bundle: %w", err)
		}
		defer zst.Close()
		reader = zst
	}

	tr := tar.NewReader(reader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}

		target := filepath.Join(dst, filepath.Clean(hdr.Name))
		if !strings.HasPrefix(target, filepath.Clean(dst)+string(os.PathSeparator)) && filepath.Clean(target) != filepath.Clean(dst) {
			return fmt.Errorf("invalid tar path: %q", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create dir %q: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("create parent dir %q: %w", filepath.Dir(target), err)
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return fmt.Errorf("create file %q: %w", target, err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return fmt.Errorf("write file %q: %w", target, err)
			}
			if err := out.Close(); err != nil {
				return fmt.Errorf("close file %q: %w", target, err)
			}
		}
	}

	cookbooks := filepath.Join(dst, "cookbooks")
	if info, err := os.Stat(cookbooks); err != nil || !info.IsDir() {
		return fmt.Errorf("bundle missing required cookbooks directory")
	}
	return nil
}

func cacheMetadataFromDigest(src config.RemoteSource, digest string) cacheMetadata {
	now := time.Now().UTC()
	meta := cacheMetadata{Digest: digest, FetchedAt: now, SourceURL: src.URL}
	if src.MaxCacheAge != "" {
		if d, err := time.ParseDuration(src.MaxCacheAge); err == nil {
			meta.ExpiresAt = now.Add(d)
		}
	}
	return meta
}

func shouldRefresh(meta cacheMetadata, refreshInterval string) bool {
	if refreshInterval == "" {
		return true
	}
	interval, err := time.ParseDuration(refreshInterval)
	if err != nil {
		return true
	}
	return time.Since(meta.FetchedAt) >= interval
}

func isCacheStale(meta cacheMetadata, maxCacheAge string) (bool, time.Duration, error) {
	age := time.Since(meta.FetchedAt)
	if maxCacheAge == "" {
		return false, age, nil
	}
	maxAge, err := time.ParseDuration(maxCacheAge)
	if err != nil {
		return false, age, err
	}
	return age > maxAge, age, nil
}

func loadCurrentMetadata(cacheDir string) (*cacheMetadata, string, error) {
	path := filepath.Join(cacheDir, "metadata", "current.json")
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var meta cacheMetadata
	if err := json.Unmarshal(bytes, &meta); err != nil {
		return nil, "", err
	}
	bundlePath := filepath.Join(cacheDir, "bundles", meta.Digest, "cookbooks")
	if info, err := os.Stat(bundlePath); err != nil || !info.IsDir() {
		return nil, "", fmt.Errorf("cached bundle path unavailable")
	}
	return &meta, bundlePath, nil
}

func writeCurrentMetadata(cacheDir string, meta cacheMetadata) (*cacheMetadata, error) {
	metadataDir := filepath.Join(cacheDir, "metadata")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create metadata dir: %w", err)
	}
	bytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode metadata: %w", err)
	}
	tmp := filepath.Join(metadataDir, "current.json.tmp")
	if err := os.WriteFile(tmp, bytes, 0o644); err != nil {
		return nil, fmt.Errorf("write temp metadata: %w", err)
	}
	if err := os.Rename(tmp, filepath.Join(metadataDir, "current.json")); err != nil {
		return nil, fmt.Errorf("activate metadata: %w", err)
	}
	return &meta, nil
}
