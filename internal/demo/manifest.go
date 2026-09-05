//go:build demo

package demo

import (
	"embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

//go:embed assets
var assetsFS embed.FS

// manifestVersion describes one extra version layered on top of an asset's
// base file, in seed order (the last entry becomes the current version).
type manifestVersion struct {
	Comment string `yaml:"comment"`
}

// manifestEntry is one asset in manifest.yaml.
type manifestEntry struct {
	Name        string            `yaml:"name"`
	File        string            `yaml:"file"`
	Generator   string            `yaml:"generator"`
	CropOf      string            `yaml:"crop_of"`
	Project     string            `yaml:"project"`
	Folder      string            `yaml:"folder"`
	Mime        string            `yaml:"mime"`
	Width       int               `yaml:"width"`
	Height      int               `yaml:"height"`
	BGColor     string            `yaml:"bg_color"`
	Tags        []string          `yaml:"tags"`
	Fields      map[string]string `yaml:"fields"`
	Alt         string            `yaml:"alt"`
	Description string            `yaml:"description"`
	Versions    []manifestVersion `yaml:"versions"`
}

type manifestFile struct {
	Assets []manifestEntry `yaml:"assets"`
}

var validProjects = map[string]bool{
	"brand": true, "summer": true, "website": true, "archive": true,
}

var validFolders = map[string]bool{
	"": true, "logos": true, "colors": true, "photo": true, "video": true,
	"social": true, "wireframes": true, "ui": true, "exports": true, "printready": true,
}

// validTags is the fixed DM-1.6 tag vocabulary. Keep in sync with seedTags.
var validTags = map[string]bool{
	"approved": true, "hero": true, "social": true, "print": true, "web": true,
	"draft": true, "archive": true, "photography": true, "video": true, "brand": true,
}

// validFieldKeys is the fixed DM-1.4 asset-scope field vocabulary.
var validFieldKeys = map[string]bool{
	"client": true, "status": true, "usage_rights": true, "photographer": true, "licensed": true,
}

var validGenerators = map[string]bool{
	"": true, "wireframe": true, "pdf": true, "svg": true, "zip": true,
}

// loadManifest parses and validates assets/manifest.yaml, failing fast with a
// clear error on any reference a maintainer could get wrong by hand: an
// unknown project/folder/tag/field key, a file-backed entry whose file
// doesn't exist under assets/, or a name collision.
func loadManifest() (*manifestFile, error) {
	raw, err := assetsFS.ReadFile("assets/manifest.yaml")
	if err != nil {
		return nil, fmt.Errorf("demo: read manifest.yaml: %w", err)
	}

	mf, err := parseManifest(raw)
	if err != nil {
		return nil, err
	}
	if err := validateManifest(mf); err != nil {
		return nil, err
	}
	return mf, nil
}

// parseManifest unmarshals raw YAML into a manifestFile without validating it.
// Split out from loadManifest so tests can exercise validateManifest against
// synthetic (including deliberately broken) manifests.
func parseManifest(raw []byte) (*manifestFile, error) {
	var mf manifestFile
	if err := yaml.Unmarshal(raw, &mf); err != nil {
		return nil, fmt.Errorf("demo: parse manifest.yaml: %w", err)
	}
	return &mf, nil
}

// validateManifest checks every structural invariant a maintainer could break
// by hand-editing manifest.yaml: unknown project/folder/generator/tag/field
// keys, a missing or ambiguous file/generator source, a file-backed entry
// whose file doesn't exist under assets/, duplicate names, and empty tags.
func validateManifest(mf *manifestFile) error {
	byName := make(map[string]manifestEntry, len(mf.Assets))
	for _, e := range mf.Assets {
		byName[e.Name] = e
	}

	seen := map[string]bool{}
	for i, e := range mf.Assets {
		ref := fmt.Sprintf("manifest.yaml entry %d (%s)", i, e.Name)

		if e.Name == "" {
			return fmt.Errorf("demo: %s: missing name", ref)
		}
		if seen[e.Name] {
			return fmt.Errorf("demo: %s: duplicate asset name %q", ref, e.Name)
		}
		seen[e.Name] = true

		if !validProjects[e.Project] {
			return fmt.Errorf("demo: %s: unknown project %q", ref, e.Project)
		}
		if !validFolders[e.Folder] {
			return fmt.Errorf("demo: %s: unknown folder %q", ref, e.Folder)
		}
		if !validGenerators[e.Generator] {
			return fmt.Errorf("demo: %s: unknown generator %q", ref, e.Generator)
		}
		sources := 0
		if e.File != "" {
			sources++
		}
		if e.Generator != "" {
			sources++
		}
		if e.CropOf != "" {
			sources++
		}
		if sources == 0 {
			return fmt.Errorf("demo: %s: needs one of file, generator, or crop_of", ref)
		}
		if sources > 1 {
			return fmt.Errorf("demo: %s: file, generator, and crop_of are mutually exclusive", ref)
		}
		if e.File != "" {
			if _, err := assetsFS.ReadFile("assets/" + e.File); err != nil {
				return fmt.Errorf("demo: %s: file %q not found under assets/: %w", ref, e.File, err)
			}
		}
		if e.CropOf != "" {
			src, ok := byName[e.CropOf]
			if !ok {
				return fmt.Errorf("demo: %s: crop_of %q does not match any asset name", ref, e.CropOf)
			}
			if src.File == "" {
				return fmt.Errorf("demo: %s: crop_of %q must reference a file-backed asset", ref, e.CropOf)
			}
		}
		if len(e.Tags) == 0 {
			return fmt.Errorf("demo: %s: must have at least one tag", ref)
		}
		for _, t := range e.Tags {
			if !validTags[t] {
				return fmt.Errorf("demo: %s: unknown tag %q", ref, t)
			}
		}
		for k := range e.Fields {
			if !validFieldKeys[k] {
				return fmt.Errorf("demo: %s: unknown field key %q", ref, k)
			}
		}
	}

	return nil
}
