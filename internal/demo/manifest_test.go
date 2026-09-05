//go:build demo

package demo

import (
	"slices"
	"testing"
)

func TestLoadManifest_Valid(t *testing.T) {
	mf, err := loadManifest()
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	if len(mf.Assets) == 0 {
		t.Fatal("expected at least one asset in manifest.yaml")
	}
}

// TestLoadManifest_StructuralInvariants checks the RA-4 correctness rules
// against the real manifest.yaml: hero-tagged entries are photos, video-tagged
// entries have a video MIME type, and every entry has at least one tag drawn
// from the fixed vocabulary (the vocabulary/no-empty-tags checks already run
// inside loadManifest, so a failure here means the manifest and this test
// have drifted, not that validation is missing).
func TestLoadManifest_StructuralInvariants(t *testing.T) {
	mf, err := loadManifest()
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}

	for _, e := range mf.Assets {
		hasTag := func(name string) bool {
			return slices.Contains(e.Tags, name)
		}

		if hasTag("hero") && e.Mime != "image/jpeg" && e.Mime != "image/png" {
			t.Errorf("asset %q is tagged hero but has mime %q, not a photo", e.Name, e.Mime)
		}
		if hasTag("video") && e.Mime != "video/mp4" {
			t.Errorf("asset %q is tagged video but has mime %q", e.Name, e.Mime)
		}
		if len(e.Tags) == 0 {
			t.Errorf("asset %q has no tags", e.Name)
		}
	}
}

func TestValidateManifest_RejectsUnknownProject(t *testing.T) {
	mf := &manifestFile{Assets: []manifestEntry{
		{
			Name:    "broken.jpg",
			File:    "photos/hero-shot-beach.jpg",
			Project: "not-a-real-project",
			Mime:    "image/jpeg",
			Tags:    []string{"approved"},
		},
	}}
	if err := validateManifest(mf); err == nil {
		t.Fatal("expected error for unknown project, got nil")
	}
}

func TestValidateManifest_RejectsMissingFile(t *testing.T) {
	mf := &manifestFile{Assets: []manifestEntry{
		{
			Name:    "broken.jpg",
			File:    "photos/does-not-exist.jpg",
			Project: "brand",
			Mime:    "image/jpeg",
			Tags:    []string{"approved"},
		},
	}}
	err := validateManifest(mf)
	if err == nil {
		t.Fatal("expected error for missing file reference, got nil")
	}
}

func TestValidateManifest_RejectsUnknownTag(t *testing.T) {
	mf := &manifestFile{Assets: []manifestEntry{
		{
			Name:    "broken.jpg",
			File:    "photos/hero-shot-beach.jpg",
			Project: "brand",
			Mime:    "image/jpeg",
			Tags:    []string{"Hero"},
		},
	}}
	if err := validateManifest(mf); err == nil {
		t.Fatal("expected error for unknown tag casing, got nil")
	}
}

func TestValidateManifest_RejectsUnknownField(t *testing.T) {
	mf := &manifestFile{Assets: []manifestEntry{
		{
			Name: "broken.jpg", File: "photos/hero-shot-beach.jpg", Project: "brand", Mime: "image/jpeg",
			Tags: []string{"approved"}, Fields: map[string]string{"not_a_field": "x"},
		},
	}}
	if err := validateManifest(mf); err == nil {
		t.Fatal("expected error for unknown field key, got nil")
	}
}

func TestValidateManifest_RejectsZeroTags(t *testing.T) {
	mf := &manifestFile{Assets: []manifestEntry{
		{Name: "broken.jpg", File: "photos/hero-shot-beach.jpg", Project: "brand", Mime: "image/jpeg"},
	}}
	if err := validateManifest(mf); err == nil {
		t.Fatal("expected error for zero tags, got nil")
	}
}

func TestValidateManifest_RejectsDuplicateName(t *testing.T) {
	mf := &manifestFile{Assets: []manifestEntry{
		{
			Name:    "dup.jpg",
			File:    "photos/hero-shot-beach.jpg",
			Project: "brand",
			Mime:    "image/jpeg",
			Tags:    []string{"approved"},
		},
		{
			Name:    "dup.jpg",
			File:    "photos/hero-shot-studio.jpg",
			Project: "brand",
			Mime:    "image/jpeg",
			Tags:    []string{"approved"},
		},
	}}
	if err := validateManifest(mf); err == nil {
		t.Fatal("expected error for duplicate asset name, got nil")
	}
}

func TestValidateManifest_RejectsFileAndGeneratorTogether(t *testing.T) {
	mf := &manifestFile{Assets: []manifestEntry{
		{
			Name:      "broken.jpg",
			File:      "photos/hero-shot-beach.jpg",
			Generator: "pdf",
			Project:   "brand",
			Mime:      "image/jpeg",
			Tags:      []string{"approved"},
		},
	}}
	if err := validateManifest(mf); err == nil {
		t.Fatal("expected error for both file and generator set, got nil")
	}
}
