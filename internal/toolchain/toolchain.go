// Package toolchain manages the melange + apko binary lifecycle.
//
// Instead of requiring users to install melange and apko manually,
// nimbopacks can download, verify, and manage them automatically.
// This is similar to how Gradle wraps itself, or rustup manages Rust versions.
//
// Binaries are stored in ~/.nimbopacks/toolchain/bin/ and versioned so
// multiple nimbopacks projects can pin different tool versions if needed.
//
// Usage:
//
//	nimbopacks toolchain install          # install latest melange + apko
//	nimbopacks toolchain install --version 0.14.0  # pin a version
//	nimbopacks toolchain status           # show installed versions
//	nimbopacks toolchain upgrade          # upgrade to latest
//	nimbopacks toolchain remove           # clean up
package toolchain

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	melangeRepo    = "chainguard-dev/melange"
	apkoRepo       = "chainguard-dev/apko"
	grypeRepo      = "anchore/grype"
	githubAPI      = "https://api.github.com/repos"
	maxBinaryBytes = 500 << 20 // 500 MiB
)

// ToolInfo describes an installed tool.
type ToolInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Path      string `json:"path"`
	Available bool   `json:"available"`
}

// Status returns information about the current toolchain state.
type Status struct {
	Melange    ToolInfo `json:"melange"`
	Apko       ToolInfo `json:"apko"`
	Grype      ToolInfo `json:"grype"`
	InstallDir string   `json:"install_dir"`
}

// BaseDir returns the nimbopacks home directory.
func BaseDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".nimbopacks")
}

// BinDir returns the managed binary directory.
func BinDir() string {
	return filepath.Join(BaseDir(), "toolchain", "bin")
}

// VersionFile returns the path to the version tracking file.
func VersionFile() string {
	return filepath.Join(BaseDir(), "toolchain", "versions.json")
}

// GetStatus checks what's currently installed.
func GetStatus() (*Status, error) {
	status := &Status{
		InstallDir: BinDir(),
	}

	status.Melange = checkTool("melange")
	status.Apko = checkTool("apko")
	status.Grype = checkTool("grype")

	return status, nil
}

func checkTool(name string) ToolInfo {
	info := ToolInfo{Name: name}

	// Check managed path first.
	managed := filepath.Join(BinDir(), name)
	if _, err := os.Stat(managed); err == nil {
		info.Path = managed
		info.Available = true
		info.Version = getVersion(managed)
		return info
	}

	// Check system PATH.
	if path, err := exec.LookPath(name); err == nil {
		info.Path = path
		info.Available = true
		info.Version = getVersion(path)
		return info
	}

	info.Available = false
	return info
}

func getVersion(path string) string {
	name := filepath.Base(path)
	flag := "version"
	if name == "grype" {
		flag = "--version"
	}
	out, err := exec.CommandContext(context.Background(), path, flag).CombinedOutput()
	if err != nil {
		return "unknown"
	}
	return normalizeVersion(strings.TrimSpace(string(out)))
}

// normalizeVersion extracts and normalizes a semver tag from a tool's version
// output. Tools print things like "melange version v0.20.1" or "grype 0.80.0";
// this returns a consistent "vX.Y.Z" string so Upgrade() can compare with
// GitHub release tags, which always use the "v" prefix.
func normalizeVersion(raw string) string {
	// Find first token that looks like a version (optional v + digits.digits.digits).
	for token := range strings.FieldsSeq(raw) {
		t := strings.TrimPrefix(token, "v")
		parts := strings.SplitN(t, ".", 3)
		if len(parts) >= 2 {
			// Looks like a semver — check digits.
			valid := true
			for _, p := range parts {
				for _, c := range strings.TrimRight(p, "-+abcdefghijklmnopqrstuvwxyz") {
					if c < '0' || c > '9' {
						valid = false
						break
					}
				}
			}
			if valid && len(parts[0]) > 0 && len(parts[1]) > 0 {
				return "v" + t
			}
		}
	}
	return raw
}

// Install downloads and installs melange and apko.
func Install(ctx context.Context, version string) error {
	binDir := BinDir()
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("creating toolchain dir: %w", err)
	}

	fmt.Println("Installing nimbopacks toolchain...")
	fmt.Printf("  Directory: %s\n\n", binDir)

	// Install melange.
	fmt.Println("  [1/3] melange")
	melangeVer := version
	if melangeVer == "" {
		var err error
		melangeVer, err = getLatestRelease(ctx, melangeRepo)
		if err != nil {
			return fmt.Errorf("fetching melange version: %w", err)
		}
	}
	if err := downloadRelease(ctx, melangeRepo, melangeVer, "melange", binDir); err != nil {
		return fmt.Errorf("installing melange: %w", err)
	}
	fmt.Printf("  ✓ melange %s\n\n", melangeVer)

	// Install apko.
	fmt.Println("  [2/3] apko")
	apkoVer := version
	if apkoVer == "" {
		var err error
		apkoVer, err = getLatestRelease(ctx, apkoRepo)
		if err != nil {
			return fmt.Errorf("fetching apko version: %w", err)
		}
	}
	if err := downloadRelease(ctx, apkoRepo, apkoVer, "apko", binDir); err != nil {
		return fmt.Errorf("installing apko: %w", err)
	}
	fmt.Printf("  ✓ apko %s\n\n", apkoVer)

	// Install grype.
	fmt.Println("  [3/3] grype")
	grypeVer := version
	if grypeVer == "" {
		var err error
		grypeVer, err = getLatestRelease(ctx, grypeRepo)
		if err != nil {
			return fmt.Errorf("fetching grype version: %w", err)
		}
	}
	if err := downloadRelease(ctx, grypeRepo, grypeVer, "grype", binDir); err != nil {
		return fmt.Errorf("installing grype: %w", err)
	}
	fmt.Printf("  ✓ grype %s\n\n", grypeVer)

	// Save version info.
	versions := map[string]string{
		"melange":      melangeVer,
		"apko":         apkoVer,
		"grype":        grypeVer,
		"installed_at": time.Now().Format(time.RFC3339),
	}
	versData, _ := json.MarshalIndent(versions, "", "  ")
	if err := os.WriteFile(VersionFile(), versData, 0o644); err != nil {
		fmt.Printf("  warning: failed to save version info: %v\n", err)
	}

	fmt.Println("  ✓ Toolchain installed successfully.")
	fmt.Printf("\n  Add to PATH: export PATH=\"%s:$PATH\"\n", binDir)
	fmt.Println("  Or nimbopacks will use it automatically.")

	return nil
}

// Upgrade checks for and installs newer versions.
func Upgrade(ctx context.Context) error {
	fmt.Println("Checking for toolchain updates...")

	melangeLatest, err := getLatestRelease(ctx, melangeRepo)
	if err != nil {
		return err
	}
	apkoLatest, err := getLatestRelease(ctx, apkoRepo)
	if err != nil {
		return err
	}

	status, _ := GetStatus()

	needsUpdate := false
	if status.Melange.Version != melangeLatest {
		fmt.Printf("  melange: %s → %s\n", status.Melange.Version, melangeLatest)
		needsUpdate = true
	}
	if status.Apko.Version != apkoLatest {
		fmt.Printf("  apko: %s → %s\n", status.Apko.Version, apkoLatest)
		needsUpdate = true
	}
	grypeLatest, err := getLatestRelease(ctx, grypeRepo)
	if err != nil {
		return err
	}
	if status.Grype.Version != grypeLatest {
		fmt.Printf("  grype: %s → %s\n", status.Grype.Version, grypeLatest)
		needsUpdate = true
	}

	if !needsUpdate {
		fmt.Println("  ✓ Everything is up to date.")
		return nil
	}

	return Install(ctx, "")
}

// Remove deletes the managed toolchain.
func Remove() error {
	dir := filepath.Join(BaseDir(), "toolchain")
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	fmt.Printf("  ✓ Removed %s\n", dir)
	return nil
}

// FormatStatus produces a human-readable status report.
func FormatStatus(s *Status) string {
	var buf strings.Builder
	buf.WriteString("Nimbopacks Toolchain Status\n\n")
	fmt.Fprintf(&buf, "  Install dir: %s\n\n", s.InstallDir)

	formatTool := func(t ToolInfo) {
		if t.Available {
			fmt.Fprintf(&buf, "  ✓ %-10s %s\n", t.Name, t.Version)
			fmt.Fprintf(&buf, "    Path: %s\n", t.Path)
		} else {
			fmt.Fprintf(&buf, "  ✗ %-10s not installed\n", t.Name)
		}
	}

	formatTool(s.Melange)
	formatTool(s.Apko)
	formatTool(s.Grype)

	if !s.Melange.Available || !s.Apko.Available || !s.Grype.Available {
		buf.WriteString("\n  Run `nimbopacks toolchain install` to install missing tools.\n")
	}

	return buf.String()
}

// --- GitHub release helpers ---

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// setGitHubAuth attaches a token to GitHub API requests when one is
// available (GITHUB_TOKEN, e.g. from GitHub Actions, or GH_TOKEN as used by
// gh/gh-cli). Unauthenticated requests are capped at 60/hour per IP, which
// CI matrix jobs blow through in minutes and get 401/403s back; an
// authenticated request raises that to 5000/hour.
func setGitHubAuth(req *http.Request) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func getLatestRelease(ctx context.Context, repo string) (string, error) {
	url := fmt.Sprintf("%s/%s/releases/latest", githubAPI, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	setGitHubAuth(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned %d for %s", resp.StatusCode, repo)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return release.TagName, nil
}

func downloadRelease(ctx context.Context, repo, version, toolName, destDir string) error {
	// Determine the asset name based on OS/arch.
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Common naming patterns across Chainguard (melange, apko) and Anchore (grype) releases.
	assetPatterns := []string{
		fmt.Sprintf("%s_%s_%s_%s", toolName, strings.TrimPrefix(version, "v"), goos, goarch),
		fmt.Sprintf("%s_%s_%s", toolName, goos, goarch),
		fmt.Sprintf("%s-%s-%s", toolName, goos, goarch),
	}

	// Fetch release assets.
	url := fmt.Sprintf("%s/%s/releases/tags/%s", githubAPI, repo, version)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	setGitHubAuth(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return err
	}

	// Find matching asset — skip package-manager formats and checksums/signatures.
	skipSuffixes := []string{".deb", ".rpm", ".apk", ".msi", ".pkg", ".sha256", ".pem", ".sig", ".sbom"}
	isSkippable := func(name string) bool {
		lower := strings.ToLower(name)
		for _, ext := range skipSuffixes {
			if strings.HasSuffix(lower, ext) {
				return true
			}
		}
		return false
	}

	var downloadURL string
	for _, asset := range release.Assets {
		if isSkippable(asset.Name) {
			continue
		}
		for _, pattern := range assetPatterns {
			if strings.Contains(strings.ToLower(asset.Name), strings.ToLower(pattern)) {
				downloadURL = asset.BrowserDownloadURL
				break
			}
		}
		if downloadURL != "" {
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no matching release asset for %s %s/%s in %s %s",
			toolName, goos, goarch, repo, version)
	}

	// Download.
	fmt.Printf("    Downloading %s...\n", filepath.Base(downloadURL))
	dlReq, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return err
	}

	dlResp, err := http.DefaultClient.Do(dlReq)
	if err != nil {
		return err
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: HTTP %d", filepath.Base(downloadURL), dlResp.StatusCode)
	}

	destPath := filepath.Join(destDir, toolName)

	if strings.HasSuffix(strings.ToLower(filepath.Base(downloadURL)), ".tar.gz") {
		if err := extractFromTarGz(dlResp.Body, toolName, destPath); err != nil {
			return fmt.Errorf("extracting %s: %w", toolName, err)
		}
	} else {
		f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(f, io.LimitReader(dlResp.Body, maxBinaryBytes)); err != nil {
			return err
		}
	}

	return nil
}

// extractFromTarGz extracts a named binary from a .tar.gz archive body.
func extractFromTarGz(r io.Reader, binaryName, destPath string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// Match the binary by name (handles paths like "grype" or "./grype").
		if filepath.Base(hdr.Name) == binaryName && hdr.Typeflag == tar.TypeReg {
			f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, io.LimitReader(tr, maxBinaryBytes))
			f.Close()
			return err
		}
	}
	return fmt.Errorf("binary %q not found in archive", binaryName)
}
