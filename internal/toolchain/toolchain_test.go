package toolchain

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// fakeToolArchive builds a .tar.gz containing a single executable named
// binaryName with the given content, mirroring the shape of a real
// melange/apko/grype release asset.
func fakeToolArchive(t *testing.T, binaryName string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: binaryName, Mode: 0o755, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDownloadReleaseChecksumVerification(t *testing.T) {
	archive := fakeToolArchive(t, "mytool", []byte("#!/bin/sh\necho hi\n"))
	sum := sha256.Sum256(archive)
	goodDigest := hex.EncodeToString(sum[:])

	mux := http.NewServeMux()
	mux.HandleFunc("/assets/mytool_1.0.0_linux_amd64.tar.gz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mux.HandleFunc("/repos/acme/mytool/releases/tags/v1.0.0", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(ghRelease{
			TagName: "v1.0.0",
			Assets: []ghAsset{
				{Name: "mytool_1.0.0_linux_amd64.tar.gz", BrowserDownloadURL: srv.URL + "/assets/mytool_1.0.0_linux_amd64.tar.gz"},
			},
		})
	})

	origAPI := githubAPI
	githubAPI = srv.URL + "/repos"
	defer func() { githubAPI = origAPI }()

	destDir := t.TempDir()

	t.Run("matching checksum installs the binary", func(t *testing.T) {
		if err := downloadRelease(t.Context(), "acme/mytool", "v1.0.0", "mytool", destDir, goodDigest); err != nil {
			t.Fatalf("downloadRelease: %v", err)
		}
		if _, err := os.Stat(filepath.Join(destDir, "mytool")); err != nil {
			t.Fatalf("expected binary to be installed: %v", err)
		}
	})

	t.Run("mismatched checksum is rejected", func(t *testing.T) {
		destDir := t.TempDir()
		err := downloadRelease(t.Context(), "acme/mytool", "v1.0.0", "mytool", destDir, "0000000000000000000000000000000000000000000000000000000000000000")
		if err == nil {
			t.Fatal("expected a checksum mismatch error, got nil")
		}
		if _, statErr := os.Stat(filepath.Join(destDir, "mytool")); statErr == nil {
			t.Fatal("binary should not be installed when checksum verification fails")
		}
	})
}

func TestNormalizeVersion(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		// melange prints "melange version v0.20.1" or "melange v0.20.1 ..."
		{"melange version v0.20.1", "v0.20.1"},
		{"melange v0.20.1 (commit abc123)", "v0.20.1"},
		// apko prints "apko version v0.27.0"
		{"apko version v0.27.0", "v0.27.0"},
		// grype --version prints "grype 0.80.0"
		{"grype 0.80.0", "v0.80.0"},
		// already normalized
		{"v0.20.1", "v0.20.1"},
		// no match → return as-is
		{"unknown", "unknown"},
	}
	for _, tc := range cases {
		got := normalizeVersion(tc.input)
		if got != tc.want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
