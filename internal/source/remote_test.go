package source

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"

	"sushi/internal/config"
)

func TestValidateRemoteSecurityPolicy(t *testing.T) {
	tests := []struct {
		name      string
		src       config.RemoteSource
		wantError string
	}{
		{name: "good https with checksum", src: config.RemoteSource{URL: "https://example.org/cookbooks.tar", ChecksumURL: "https://example.org/cookbooks.sha256"}},
		{name: "bad source url", src: config.RemoteSource{URL: "://bad"}, wantError: "invalid remote URL"},
		{name: "bad checksum url", src: config.RemoteSource{URL: "https://example.org/cookbooks.tar", ChecksumURL: "://bad", SkipChecksum: false}, wantError: "invalid checksum URL"},
		{name: "http source requires allow_insecure", src: config.RemoteSource{URL: "http://example.org/cookbooks.tar", SkipChecksum: true}, wantError: "allow_insecure"},
		{name: "skip_checksum false needs checksum", src: config.RemoteSource{URL: "https://example.org/cookbooks.tar", SkipChecksum: false}, wantError: "skip_checksum"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateRemoteSecurityPolicy(tc.src)
			if tc.wantError == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("expected error containing %q, got %v", tc.wantError, err)
				}
			}
		})
	}
}

func TestFetchAndActivateRemoteChecksumAndCompression(t *testing.T) {
	baseTar := makeRemoteTar(t)
	gzipBundle := compressGzip(t, baseTar)
	zstdBundle := compressZstd(t, baseTar)
	goodChecksum := hexDigest(gzipBundle)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bundle.tar.gz", "/bundle.tgz":
			_, _ = w.Write(gzipBundle)
		case "/bundle.tar.zst", "/bundle.tar.rst":
			_, _ = w.Write(zstdBundle)
		case "/checksum.good":
			_, _ = w.Write([]byte(goodChecksum + "\n"))
		case "/checksum.bad":
			_, _ = w.Write([]byte("00ff\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Run("good checksum", func(t *testing.T) {
		src := config.RemoteSource{URL: server.URL + "/bundle.tar.gz", ChecksumURL: server.URL + "/checksum.good", AllowInsecure: true, CacheDir: filepath.Join(t.TempDir(), "cache")}
		got, err := fetchAndActivateRemote(src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.meta.Digest == "" {
			t.Fatal("expected digest")
		}
	})

	t.Run("bad checksum", func(t *testing.T) {
		src := config.RemoteSource{URL: server.URL + "/bundle.tar.gz", ChecksumURL: server.URL + "/checksum.bad", AllowInsecure: true, CacheDir: filepath.Join(t.TempDir(), "cache")}
		_, err := fetchAndActivateRemote(src)
		if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
			t.Fatalf("expected checksum mismatch, got %v", err)
		}
	})

	for _, path := range []string{"/bundle.tgz", "/bundle.tar.zst", "/bundle.tar.rst"} {
		path := path
		t.Run("supports "+path, func(t *testing.T) {
			src := config.RemoteSource{URL: server.URL + path, AllowInsecure: true, SkipChecksum: true, CacheDir: filepath.Join(t.TempDir(), "cache")}
			got, err := fetchAndActivateRemote(src)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.bundlePath == "" {
				t.Fatal("expected bundle path")
			}
		})
	}
}

func makeRemoteTar(t *testing.T) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	tw := tar.NewWriter(buf)
	entries := []struct{ name, body string }{{"cookbooks/", ""}, {"cookbooks/example/", ""}, {"cookbooks/example/metadata.rb", "name 'example'"}}
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Mode: 0o755, Typeflag: tar.TypeDir}
		if e.body != "" {
			h.Typeflag = tar.TypeReg
			h.Mode = 0o644
			h.Size = int64(len(e.body))
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatalf("write header: %v", err)
		}
		if e.body != "" {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatalf("write body: %v", err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	return buf.Bytes()
}

func compressGzip(t *testing.T, data []byte) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	if _, err := gz.Write(data); err != nil {
		t.Fatalf("write gzip: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func compressZstd(t *testing.T, data []byte) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	zw, err := zstd.NewWriter(buf)
	if err != nil {
		t.Fatalf("new zstd: %v", err)
	}
	if _, err := zw.Write(data); err != nil {
		t.Fatalf("write zstd: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zstd: %v", err)
	}
	return buf.Bytes()
}

func hexDigest(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
