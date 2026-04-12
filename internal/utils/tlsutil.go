// Package tlsutil handles custom CA certificates across the entire nimbopacks pipeline.
//
// Custom CAs need to work in five places:
//
//  1. Toolchain downloads (fetching melange/apko from GitHub)
//  2. Melange builds (package downloads inside bubblewrap sandbox)
//  3. Apko image assembly (pulling base packages from Wolfi repos)
//  4. Registry pushes (pushing to private registries with self-signed certs)
//  5. The final image (so the running app trusts the same CAs)
//
// This package provides helpers for all five scenarios.
package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Nimbostack/nimbopacks/internal/types"
)

// LoadCACerts reads all custom CA certificates from the TLS config
// and returns a combined PEM bundle.
func LoadCACerts(srcDir string, cfg types.TLSConfig) ([]byte, error) {
	var combined []byte

	// Load individual cert files.
	for _, certPath := range cfg.CACertPaths {
		resolved := resolvePath(srcDir, certPath)
		data, err := os.ReadFile(resolved)
		if err != nil {
			return nil, fmt.Errorf("reading CA cert %s: %w", certPath, err)
		}
		combined = append(combined, data...)
		if !strings.HasSuffix(string(data), "\n") {
			combined = append(combined, '\n')
		}
	}

	// Load all certs from a directory.
	if cfg.CADirPath != "" {
		resolved := resolvePath(srcDir, cfg.CADirPath)
		entries, err := os.ReadDir(resolved)
		if err != nil {
			return nil, fmt.Errorf("reading CA dir %s: %w", cfg.CADirPath, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".pem") && !strings.HasSuffix(name, ".crt") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(resolved, name))
			if err != nil {
				return nil, fmt.Errorf("reading CA cert %s: %w", name, err)
			}
			combined = append(combined, data...)
			if !strings.HasSuffix(string(data), "\n") {
				combined = append(combined, '\n')
			}
		}
	}

	return combined, nil
}

// NewTLSConfig creates a *tls.Config that trusts both system CAs
// and any custom CAs from the nimbopacks config.
// Used by the toolchain manager, updater, and registry push operations.
func NewTLSConfig(srcDir string, cfg types.TLSConfig) (*tls.Config, error) {
	if cfg.Insecure {
		return &tls.Config{InsecureSkipVerify: true}, nil //nolint:gosec // intentional: user explicitly set tls.insecure = true
	}

	if !cfg.HasCustomCAs() {
		return nil, nil // Use system defaults.
	}

	// Start with system CA pool.
	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}

	// Add custom CAs.
	pemData, err := LoadCACerts(srcDir, cfg)
	if err != nil {
		return nil, err
	}
	if len(pemData) > 0 {
		if !pool.AppendCertsFromPEM(pemData) {
			return nil, errors.New("failed to parse custom CA certificates")
		}
	}

	return &tls.Config{RootCAs: pool}, nil
}

// WriteBundleToWorkDir writes the combined CA bundle to a temp file
// so it can be mounted into melange/apko builds.
// Returns the path to the bundle file.
func WriteBundleToWorkDir(workDir, srcDir string, cfg types.TLSConfig) (string, error) {
	if !cfg.HasCustomCAs() {
		return "", nil
	}

	pemData, err := LoadCACerts(srcDir, cfg)
	if err != nil {
		return "", err
	}
	if len(pemData) == 0 {
		return "", nil
	}

	bundlePath := filepath.Join(workDir, "custom-ca-bundle.pem")
	if err := os.WriteFile(bundlePath, pemData, 0o644); err != nil {
		return "", fmt.Errorf("writing CA bundle: %w", err)
	}

	return bundlePath, nil
}

// MelangeEnvVars returns environment variables that tell melange's
// build sandbox to trust the custom CAs.
func MelangeEnvVars(bundlePath string) map[string]string {
	if bundlePath == "" {
		return nil
	}
	return map[string]string{
		"SSL_CERT_FILE":       bundlePath,
		"REQUESTS_CA_BUNDLE":  bundlePath, // Python requests
		"NODE_EXTRA_CA_CERTS": bundlePath, // Node.js
		"CURL_CA_BUNDLE":      bundlePath, // curl
		"GIT_SSL_CAINFO":      bundlePath, // git
	}
}

// ImageCACertInstallStep returns a melange pipeline step that installs
// custom CAs into the image's trust store so the running app trusts them.
func ImageCACertInstallStep(bundlePath string) *types.MelangePipelineStep {
	if bundlePath == "" {
		return nil
	}
	return &types.MelangePipelineStep{
		Runs: fmt.Sprintf(`# Install custom CA certificates into the image trust store
mkdir -p /home/build/output/usr/local/share/ca-certificates
cp %s /home/build/output/usr/local/share/ca-certificates/custom-ca.crt
`, bundlePath),
	}
}

// resolvePath makes a cert path absolute, resolving relative paths against srcDir.
func resolvePath(srcDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(srcDir, path)
}
