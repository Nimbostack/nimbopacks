// Package tlsutil handles custom CA certificates across the entire nimbopacks pipeline.
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

// ImageCACertInstallStep returns a melange pipeline step that installs custom
// CAs into the image so the running app trusts them.
func ImageCACertInstallStep(bundlePath string) *types.MelangePipelineStep {
	if bundlePath == "" {
		return nil
	}
	return &types.MelangePipelineStep{
		Runs: fmt.Sprintf(`# Install custom CA certificates into the image trust locations
mkdir -p "${{targets.destdir}}/usr/local/share/ca-certificates"
cp %[1]s "${{targets.destdir}}/usr/local/share/ca-certificates/nimbopacks-custom-ca.crt"
mkdir -p "${{targets.destdir}}/etc/ssl/certs"
cp %[1]s "${{targets.destdir}}/etc/ssl/certs/nimbopacks-custom-ca.pem"
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
