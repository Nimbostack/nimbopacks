package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	// Packs
	_ "github.com/Nimbostack/nimbopacks/pkg/packs/dotnet"
	_ "github.com/Nimbostack/nimbopacks/pkg/packs/golang"
	_ "github.com/Nimbostack/nimbopacks/pkg/packs/java"
	_ "github.com/Nimbostack/nimbopacks/pkg/packs/node"
	_ "github.com/Nimbostack/nimbopacks/pkg/packs/python"
	_ "github.com/Nimbostack/nimbopacks/pkg/packs/webserver"

	// Backends
	_ "github.com/Nimbostack/nimbopacks/pkg/backend/wolfi"

	"github.com/Nimbostack/nimbopacks/internal/cache"
	"github.com/Nimbostack/nimbopacks/internal/pack/registry"
	"github.com/Nimbostack/nimbopacks/internal/pipeline"
	"github.com/Nimbostack/nimbopacks/internal/toolchain"
	"github.com/Nimbostack/nimbopacks/internal/update"
	"github.com/Nimbostack/nimbopacks/pkg/backend"
	"github.com/Nimbostack/nimbopacks/pkg/templates"
	"github.com/spf13/cobra"
)

// errCVEThreshold is returned by updateCmd when --check mode finds CVEs at or above the threshold.
// main() checks for this specific error to exit with code 1.
var errCVEThreshold = errors.New("CVEs found at or above threshold")

var version = "dev"

// promptOverwrite asks whether to overwrite an existing nimpack.yaml.
// r is the input reader (pass os.Stdin for interactive use).
// Returns true only if the user types "y" or "yes" (case-insensitive).
// Returns false, nil on EOF (non-interactive / piped context).
func promptOverwrite(path string, r io.Reader, w io.Writer) (bool, error) {
	msg := fmt.Sprintf("nimpack.yaml already exists in %s. Overwrite? [y/N]: ", filepath.Dir(path))
	return promptConfirm(msg, r, w)
}

func main() {
	if err := rootCmd().Execute(); err != nil {
		if errors.Is(err, errCVEThreshold) {
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "nimbopacks",
		Short: "Reproducible, patchable container images — zero CVE lag",
		Long: `nimbopacks builds minimal OCI images from source with full reproducibility,
			automatic SBOMs, and atomic CVE patching.

			Quick start:
			nimbopacks init go             # scaffold a nimpack.yaml
			nimbopacks build                   # build the image
			nimbopacks update --check          # check for CVEs
			nimbopacks update                  # patch and rebuild

			Auto-detect a project:
			nimbopacks detect .                # see what nimbopacks finds
			nimbopacks detect --generate .     # generate nimpack.yaml from detection`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		initCmd(),
		detectCmd(),
		buildCmd(),
		updateCmd(),
		toolchainCmd(),
		cacheCmd(),
		packsCmd(),
		templatesCmd(),
		statusCmd(),
		versionCmd(),
	)

	return root
}

// --- init ---

func initCmd() *cobra.Command {
	var generate bool
	var force bool

	cmd := &cobra.Command{
		Use:   "init [template] [source-dir]",
		Short: "Generate a nimpack.yaml from a template or detection",
		Long: `Scaffolds a nimpack.yaml configuration file.

With a template:
  nimbopacks init go           # use the go template
  nimbopacks init dotnet-solution .   # .NET monorepo in current dir

With detection:
  nimbopacks init --detect .          # auto-detect and generate config

List available templates:
  nimbopacks templates`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			srcDir := "."

			if generate || len(args) == 0 {
				// Detection-based init.
				if len(args) > 0 {
					srcDir = args[0]
				}

				return runDetectAndGenerate(srcDir, force)
			}

			templateName := args[0]
			if len(args) > 1 {
				srcDir = args[1]
			}
			return runInitFromTemplate(srcDir, templateName, force)
		},
	}

	cmd.Flags().BoolVar(&generate, "detect", false, "Use detection instead of a template")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing nimpack.yaml without prompting")
	return cmd
}

func runInitFromTemplate(srcDir, templateName string, force bool) error {
	outputPath := filepath.Join(srcDir, "nimpack.yaml")

	// Check if nimpack.yaml already exists.
	if _, err := os.Stat(outputPath); err == nil {
		if !force {
			ok, promptErr := promptOverwrite(outputPath, os.Stdin, os.Stdout)
			if promptErr != nil {
				return fmt.Errorf("reading input: %w", promptErr)
			}
			if !ok {
				fmt.Println("Aborted.")
				return nil
			}
		}
	}

	// Derive project name from directory.
	absDir, _ := filepath.Abs(srcDir)
	projectName := filepath.Base(absDir)

	// Try detection to populate variables.
	vars := templates.TemplateVars{
		ProjectName: projectName,
		Version:     "0.0.1",
	}

	match, _ := registry.DetectBest(context.Background(), srcDir)
	if match != nil {
		if ep, ok := match.Result.Metadata["entrypoint"]; ok {
			vars.Entrypoint = ep
		}
		if mod, ok := match.Result.Metadata["module_path"]; ok {
			vars.Module = mod
			parts := strings.Split(mod, "/")
			vars.ProjectName = parts[len(parts)-1]
		}
		if fw, ok := match.Result.Metadata["framework"]; ok {
			vars.Framework = fw
		}
	}

	content, err := templates.Render(templateName, vars)
	if err != nil {
		return err
	}

	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		return err
	}

	fmt.Printf("✓ Created %s (template: %s)\n", outputPath, templateName)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Review and customize nimpack.yaml")
	fmt.Println("  2. Run `nimbopacks build`")
	return nil
}

func runDetectAndGenerate(srcDir string, force bool) error {
	outputPath := filepath.Join(srcDir, "nimpack.yaml")
	if _, err := os.Stat(outputPath); err == nil {
		if !force {
			ok, promptErr := promptOverwrite(outputPath, os.Stdin, os.Stdout)
			if promptErr != nil {
				return fmt.Errorf("reading input: %w", promptErr)
			}
			if !ok {
				fmt.Println("Aborted.")
				return nil
			}
		}
	}

	match, err := registry.DetectBest(context.Background(), srcDir)
	if err != nil {
		return err
	}

	if match == nil {
		fmt.Println("Could not auto-detect project type.")
		fmt.Printf("Available templates: %s\n", strings.Join(templates.Names(), ", "))
		fmt.Println("Usage: nimbopacks init <template>")
		return nil
	}

	fmt.Printf("Detected: %s\n", match.Result.Summary)

	templateName := match.Result.SuggestedTemplate
	if templateName != "" {
		fmt.Printf("Suggested template: %s\n", templateName)
	}

	cfg, err := match.Pack.GenerateConfig(context.Background(), srcDir, &match.Result, templateName)
	if err != nil {
		return err
	}

	// Write as YAML with comments.
	content := fmt.Sprintf("# nimpack.yaml — generated by nimbopacks detect\n"+
		"# Detected: %s\n"+
		"# Template: %s\n"+
		"# Review and customize, then run: nimbopacks build\n\n",
		match.Result.Summary, templateName)

	// Use the template if available, falling back to generated config.
	if templateName != "" {
		absDir, _ := filepath.Abs(srcDir)
		vars := templates.TemplateVars{
			ProjectName: cfg.Project.Name,
			Version:     cfg.Project.Version,
		}
		if ep, ok := match.Result.Metadata["entrypoint"]; ok {
			vars.Entrypoint = ep
		}
		if mod, ok := match.Result.Metadata["module_path"]; ok {
			vars.Module = mod
			parts := strings.Split(mod, "/")
			vars.ProjectName = parts[len(parts)-1]
		} else {
			vars.ProjectName = filepath.Base(absDir)
		}
		if rendered, err := templates.Render(templateName, vars); err == nil {
			content = rendered
		}
	}

	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		return err
	}

	fmt.Printf("✓ Created %s\n", outputPath)
	return nil
}

// --- detect ---

func detectCmd() *cobra.Command {
	var generate bool
	var force bool

	cmd := &cobra.Command{
		Use:   "detect [source-dir]",
		Short: "Detect project type and suggest a template",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			srcDir := "."
			if len(args) > 0 {
				srcDir = args[0]
			}

			if generate {
				return runDetectAndGenerate(srcDir, force)
			}

			results, err := registry.DetectAll(context.Background(), srcDir)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				fmt.Println("No languages detected.")
				fmt.Printf("Available packs: %s\n", strings.Join(registry.Names(), ", "))
				return nil
			}

			fmt.Printf("Detected %d pack(s) in %s:\n\n", len(results), srcDir)
			for i, r := range results {
				marker := "  "
				if i == 0 {
					marker = "→ "
				}
				fmt.Printf("%s%-12s  %.0f%%  %s\n", marker, r.Pack.Name(), r.Result.Confidence*100, r.Result.Summary)
				if r.Result.SuggestedTemplate != "" {
					fmt.Printf("  %-12s        suggested template: %s\n", "", r.Result.SuggestedTemplate)
				}
			}

			fmt.Println("\nTo generate a config:")
			fmt.Printf("  nimbopacks init %s\n", results[0].Result.SuggestedTemplate)
			fmt.Println("  nimbopacks detect --generate")
			return nil
		},
	}

	cmd.Flags().BoolVar(&generate, "generate", false, "Generate nimpack.yaml from detection")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing nimpack.yaml without prompting (only meaningful with --generate)")
	return cmd
}

// --- build ---

func buildCmd() *cobra.Command {
	var (
		tag         string
		outputDir   string
		cfgFile     string
		push        bool
		backendName string
		caCertPath  string
	)

	cmd := &cobra.Command{
		Use:   "build [source-dir]",
		Short: "Build an OCI image from nimpack.yaml",
		Long: `Builds a minimal, reproducible OCI image.

Requires a nimpack.yaml in the source directory.
Use 'nimbopacks init' to create one.

Custom CA certificates (for corporate proxies / private registries):
  nimbopacks build --ca-cert /path/to/ca.pem
  NIMBOPACKS_CA_CERT=/path/to/ca.pem nimbopacks build
  Or configure in nimpack.yaml under tls.ca_cert_paths`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			srcDir := "."
			if len(args) > 0 {
				srcDir = args[0]
			}

			opts := pipeline.BuildOptions{
				SourceDir:   srcDir,
				ConfigPath:  cfgFile,
				Tag:         tag,
				OutputDir:   outputDir,
				Push:        push,
				BackendName: backendName,
			}

			// CLI --ca-cert flag or env var override.
			if caCertPath == "" {
				caCertPath = os.Getenv("NIMBOPACKS_CA_CERT")
			}
			if caCertPath != "" {
				opts.CACertPath = caCertPath
			}

			_, err := pipeline.Build(context.Background(), opts)
			return err
		},
	}

	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Image tag")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory")
	cmd.Flags().StringVarP(&cfgFile, "config", "c", "", "Path to nimpack.yaml")
	cmd.Flags().BoolVar(&push, "push", false, "Push to registry after build")
	cmd.Flags().StringVar(&backendName, "backend", "", "Build backend to use (default: wolfi if available)")
	cmd.Flags().StringVar(&caCertPath, "ca-cert", "", "Path to custom CA certificate (PEM). Also: NIMBOPACKS_CA_CERT env var")
	return cmd
}

// --- toolchain ---

func toolchainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "toolchain",
		Short: "Manage the melange + apko toolchain",
	}

	var toolVersion string

	install := &cobra.Command{
		Use:   "install",
		Short: "Download and install melange + apko",
		RunE: func(_ *cobra.Command, _ []string) error {
			return toolchain.Install(context.Background(), toolVersion)
		},
	}
	install.Flags().StringVar(&toolVersion, "version", "", "Pin to a specific version")

	status := &cobra.Command{
		Use:   "status",
		Short: "Show installed toolchain versions",
		RunE: func(_ *cobra.Command, _ []string) error {
			s, err := toolchain.GetStatus()
			if err != nil {
				return err
			}
			fmt.Print(toolchain.FormatStatus(s))
			return nil
		},
	}

	upgrade := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade to latest melange + apko",
		RunE: func(_ *cobra.Command, _ []string) error {
			return toolchain.Upgrade(context.Background())
		},
	}

	remove := &cobra.Command{
		Use:   "remove",
		Short: "Remove managed toolchain",
		RunE: func(_ *cobra.Command, _ []string) error {
			return toolchain.Remove()
		},
	}

	cmd.AddCommand(install, status, upgrade, remove)
	return cmd
}

// --- packs ---

func packsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "packs",
		Short: "List available language packs",
		Run: func(_ *cobra.Command, _ []string) {
			for _, name := range registry.Names() {
				fmt.Printf("  %s\n", name)
			}
		},
	}
}

// --- templates ---

func templatesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "templates",
		Short: "List available nimpack.yaml templates",
		Run: func(_ *cobra.Command, _ []string) {
			tmps := templates.List()
			if len(tmps) == 0 {
				fmt.Println("No templates registered.")
				return
			}

			currentPack := ""
			for _, t := range tmps {
				if t.Pack != currentPack {
					if currentPack != "" {
						fmt.Println()
					}
					fmt.Printf("  %s:\n", t.Pack)
					currentPack = t.Pack
				}
				fmt.Printf("    %-22s %s\n", t.Name, t.Description)
			}

			fmt.Println("\nUsage: nimbopacks init <template>")
		},
	}
}

// --- status ---

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show nimbopacks status (toolchain, backends, packs)",
		RunE: func(_ *cobra.Command, _ []string) error {
			// Toolchain.
			tc, _ := toolchain.GetStatus()
			fmt.Print(toolchain.FormatStatus(tc))

			// Backends.
			fmt.Println("\nBackends:")
			for _, name := range backend.Names() {
				b, _ := backend.Get(name)
				avail, reason := b.Available(context.Background())
				caps := backend.GetCapabilities(name)
				icon := "✓"
				if !avail {
					icon = "✗"
				}
				fmt.Printf("  %s %-12s %s\n", icon, name, reason)
				if avail {
					var features []string
					if caps.Reproducible {
						features = append(features, "reproducible")
					}
					if caps.SBOM {
						features = append(features, "SBOM")
					}
					if caps.AtomicPatching {
						features = append(features, "atomic patching")
					}
					if caps.NoExternalDeps {
						features = append(features, "no external deps")
					}
					fmt.Printf("    Features: %s\n", strings.Join(features, ", "))
				}
			}

			// Packs.
			fmt.Printf("\nPacks: %s\n", strings.Join(registry.Names(), ", "))
			fmt.Printf("Templates: %d available\n", len(templates.Names()))

			return nil
		},
	}
}

// --- update ---

func updateCmd() *cobra.Command {
	var (
		check       bool
		failOn      string
		format      string
		sbomPath    string
		outputPath  string
		grypeConfig string
		cfgFile     string
		backendName string
	)

	cmd := &cobra.Command{
		Use:   "update [source-dir]",
		Short: "Scan for CVEs and report remediation steps",
		Long: `Builds the image, then scans the apko-generated SBOM for CVEs using grype.

Check mode (for CI):
  nimbopacks update --check                    # exit 1 if CVEs found at HIGH or above
  nimbopacks update --check --fail-on critical
  nimbopacks update --check --format sarif -o results.sarif

Scan an existing SBOM (skip rebuild):
  nimbopacks update --check --sbom ./output/sbom.json

Interactive mode (non-blocking, prints remediation):
  nimbopacks update

grype policy files:
  Place .grype.yaml in your project root — grype picks it up automatically.
  Or: nimbopacks update --grype-config path/to/policy.yaml

Cache the grype vulnerability DB in CI:
  Set NIMBOPACKS_GRYPE_DB_CACHE to a persistent directory.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srcDir := "."
			if len(args) > 0 {
				srcDir = args[0]
			}

			// Env var fallbacks (only override if flag was not explicitly set).
			if !cmd.Flags().Changed("fail-on") {
				if v := os.Getenv("NIMBOPACKS_FAIL_ON"); v != "" {
					failOn = v
				}
			}
			if !cmd.Flags().Changed("format") {
				if v := os.Getenv("NIMBOPACKS_FORMAT"); v != "" {
					format = v
				}
			}
			if grypeConfig == "" {
				grypeConfig = os.Getenv("NIMBOPACKS_GRYPE_CONFIG")
			}
			dbCachePath := os.Getenv("NIMBOPACKS_GRYPE_DB_CACHE")

			opts := update.CheckOptions{
				SourceDir:   srcDir,
				ConfigPath:  cfgFile,
				SBOMPath:    sbomPath,
				GrypeConfig: grypeConfig,
				DBCachePath: dbCachePath,
				BackendName: backendName,
			}

			fmt.Println("🌩️  nimbopacks update")
			fmt.Println()

			report, err := update.Check(context.Background(), opts)
			if err != nil {
				return err
			}

			// Write report to file or stdout.
			if outputPath != "" {
				f, err := os.Create(outputPath)
				if err != nil {
					return fmt.Errorf("opening output file: %w", err)
				}
				if err := update.Format(report, format, f); err != nil {
					f.Close()
					return err
				}
				if err := f.Close(); err != nil {
					return fmt.Errorf("closing output file: %w", err)
				}
			} else {
				if err := update.Format(report, format, cmd.OutOrStdout()); err != nil {
					return err
				}
			}

			// Pinned package warnings — always to stderr.
			for _, p := range report.Packages {
				if p.Pinned {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"\n⚠  %s@%s is pinned and has %s (%s)\n   A patched version may be available. Unpin or bump in nimpack.yaml → update.pinned\n",
						p.Package, p.CurrentVersion, strings.Join(p.CVEs, ", "), p.Severity)
				}
			}

			// Return sentinel error in --check mode when CVEs meet the threshold.
			if check && update.MeetsThreshold(report, failOn) {
				fmt.Fprintf(cmd.ErrOrStderr(), "\n✗ CVEs found at or above %q threshold. See report above.\n", failOn)
				return errCVEThreshold
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&check, "check", false, "Exit 1 if CVEs found at or above --fail-on severity")
	cmd.Flags().StringVar(&failOn, "fail-on", "high", "Severity threshold for --check: critical, high, medium, low, none")
	cmd.Flags().StringVar(&format, "format", "text", "Output format: text, table, json, sarif")
	cmd.Flags().StringVar(&sbomPath, "sbom", "", "Scan an existing SBOM instead of building")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Write report to file (default: stdout)")
	cmd.Flags().StringVar(&grypeConfig, "grype-config", "", "Path to grype policy file (.grype.yaml)")
	cmd.Flags().StringVarP(&cfgFile, "config", "c", "", "Path to nimpack.yaml")
	cmd.Flags().StringVar(&backendName, "backend", "", "Build backend to use (default: wolfi if available)")

	return cmd
}

// --- version ---

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("nimbopacks %s\n", version)
		},
	}
}

// --- cache ---

func cacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage the nimbopacks build cache",
	}
	cmd.AddCommand(cacheInfoCmd(), cacheCleanCmd())
	return cmd
}

func cacheInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show build cache size and location",
		RunE: func(_ *cobra.Command, _ []string) error {
			entries, err := cache.List()
			if err != nil {
				return err
			}
			_, totalCount := cacheTotal(entries)
			if totalCount == 0 {
				fmt.Println("Cache is empty.")
				return nil
			}
			printCacheTable(entries)
			return nil
		},
	}
}

func cacheCleanCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove build cache to free disk space",
		RunE: func(_ *cobra.Command, _ []string) error {
			entries, err := cache.List()
			if err != nil {
				return err
			}

			totalSize, totalCount := cacheTotal(entries)
			if totalCount == 0 {
				fmt.Println("Nothing to clean.")
				return nil
			}

			printCacheTable(entries)
			fmt.Println()

			if !yes {
				msg := fmt.Sprintf("Remove %d items totaling %s? [y/N]: ",
					totalCount, formatBytes(totalSize))
				ok, err := promptConfirm(msg, os.Stdin, os.Stdout)
				if err != nil {
					return err
				}
				if !ok {
					fmt.Println("Aborted.")
					return nil
				}
			}

			if err := cache.Clean(entries); err != nil {
				return err
			}
			fmt.Printf("✓ Removed %s\n", formatBytes(totalSize))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func printCacheTable(entries []cache.CacheEntry) {
	fmt.Printf("Build cache: %s\n", cache.CacheDir())
	for _, e := range entries {
		fmt.Printf("  %-20s  %d items  %8s  %s\n",
			e.Name, e.Count, formatBytes(e.Size), e.Path)
	}
}

func cacheTotal(entries []cache.CacheEntry) (int64, int) {
	var size int64
	var count int
	for _, e := range entries {
		size += e.Size
		count += e.Count
	}
	return size, count
}

// promptConfirm prints msg and reads a yes/no answer from r.
// Returns true only for "y" or "yes" (case-insensitive).
// Returns false, nil on EOF (non-interactive / piped context).
func promptConfirm(msg string, r io.Reader, w io.Writer) (bool, error) {
	fmt.Fprint(w, msg)
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return false, scanner.Err()
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return answer == "y" || answer == "yes", nil
}

// formatBytes formats a byte count as a human-readable string.
func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
