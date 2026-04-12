// Package templates manages nimpack.yaml templates using Go's embed package.
//
// Templates are standalone YAML files in the templates/ directory at the repo root.
// They're embedded into the binary at compile time, so there's no runtime filesystem
// dependency — but contributors just edit YAML files.
//
// # File format
//
// Each template file has comment-based frontmatter followed by a --- separator:
//
//	# Template: go
//	# Pack: golang
//	# Description: Go REST API (gin, echo, chi, or stdlib)
//	# Tags: web, api, rest
//	---
//	schema_version: "1"
//	...
//
// # Adding a new template
//
//  1. Create a YAML file in templates/<pack>/<name>.yaml
//  2. Add frontmatter comments (Template, Pack, Description, Tags)
//  3. Write the nimpack.yaml body with {{ .ProjectName }}, {{ .Version }}, etc.
//  4. That's it — go:embed picks it up automatically at build time.
package templates

import (
	"embed"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
)

//go:embed all:templates
var templateFS embed.FS

// Template holds a parsed template and its metadata.
type Template struct {
	Name        string
	Description string
	Pack        string
	Tags        []string
	Content     string
}

// TemplateVars are values injected into templates during rendering.
type TemplateVars struct {
	ProjectName string
	Version     string
	Module      string
	Framework   string
	Entrypoint  string
}

var (
	once     sync.Once
	allTmpls map[string]*Template
	loadErr  error
)

func ensureLoaded() {
	once.Do(func() {
		allTmpls = make(map[string]*Template)
		loadErr = walkDir("templates")
		if loadErr != nil {
			fmt.Printf("warning: failed to load embedded templates: %v\n", loadErr)
		}
	})
}

func walkDir(root string) error {
	entries, err := templateFS.ReadDir(root)
	if err != nil {
		return fmt.Errorf("reading %s: %w", root, err)
	}

	for _, entry := range entries {
		path := path.Join(root, entry.Name())
		if entry.IsDir() {
			if err := walkDir(path); err != nil {
				return err
			}
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}

		data, err := templateFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		tmpl, err := parseTemplate(string(data))
		if err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}

		allTmpls[tmpl.Name] = tmpl
	}

	return nil
}

func parseTemplate(raw string) (*Template, error) {
	t := &Template{}
	lines := strings.Split(raw, "\n")
	bodyStart := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "---" {
			bodyStart = i + 1
			break
		}

		if content, ok := strings.CutPrefix(trimmed, "#"); ok {
			content = strings.TrimSpace(content)

			switch {
			case strings.HasPrefix(content, "Template:"):
				t.Name = strings.TrimSpace(strings.TrimPrefix(content, "Template:"))
			case strings.HasPrefix(content, "Pack:"):
				t.Pack = strings.TrimSpace(strings.TrimPrefix(content, "Pack:"))
			case strings.HasPrefix(content, "Description:"):
				t.Description = strings.TrimSpace(strings.TrimPrefix(content, "Description:"))
			case strings.HasPrefix(content, "Tags:"):
				for tag := range strings.SplitSeq(strings.TrimPrefix(content, "Tags:"), ",") {
					tag = strings.TrimSpace(tag)
					if tag != "" {
						t.Tags = append(t.Tags, tag)
					}
				}
			}
		}
	}

	if t.Name == "" {
		return nil, errors.New("missing # Template: header")
	}
	if t.Pack == "" {
		return nil, fmt.Errorf("template %q missing # Pack: header", t.Name)
	}

	t.Content = strings.Join(lines[bodyStart:], "\n")
	return t, nil
}

// Register adds a template programmatically.
func Register(t *Template) {
	ensureLoaded()
	allTmpls[t.Name] = t
}

// Get returns a template by name.
func Get(name string) *Template {
	ensureLoaded()
	return allTmpls[name]
}

// List returns all templates sorted by name.
func List() []*Template {
	ensureLoaded()
	out := make([]*Template, 0, len(allTmpls))
	for _, t := range allTmpls {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ListByPack returns templates for a specific pack.
func ListByPack(pack string) []*Template {
	ensureLoaded()
	var out []*Template
	for _, t := range allTmpls {
		if t.Pack == pack {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Names returns all template names sorted.
func Names() []string {
	ensureLoaded()
	out := make([]string, 0, len(allTmpls))
	for name := range allTmpls {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Render applies variables to a template.
func Render(name string, vars TemplateVars) (string, error) {
	t := Get(name)
	if t == nil {
		return "", fmt.Errorf("template %q not found; available: %s", name, strings.Join(Names(), ", "))
	}

	content := t.Content
	content = strings.ReplaceAll(content, "{{ .ProjectName }}", vars.ProjectName)
	content = strings.ReplaceAll(content, "{{ .Version }}", vars.Version)
	content = strings.ReplaceAll(content, "{{ .Module }}", vars.Module)
	content = strings.ReplaceAll(content, "{{ .Framework }}", vars.Framework)
	content = strings.ReplaceAll(content, "{{ .Entrypoint }}", vars.Entrypoint)

	return content, nil
}
