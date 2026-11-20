package skills

import (
	"octpex/pkg/protocol"
	"os"
	"testing"
)

func TestSkillCatalog_RegistrationAndHierarchy(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "octpex-skills-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cat := NewSkillCatalog(tempDir)

	skillsArm1 := []protocol.SkillManifest{
		{
			ID:             "geo_disambiguation",
			Name:           "Geographic Entity Disambiguation",
			Category:       "geospatial/normalization",
			ParentCategory: "geospatial",
			Version:        "1.0.0",
			SystemPrompt:   "Resolve geo entities...",
		},
		{
			ID:             "coordinate_converter",
			Name:           "GPS Coordinate Converter",
			Category:       "geospatial/conversion",
			ParentCategory: "geospatial",
			Version:        "1.2.0",
		},
	}

	skillsArm2 := []protocol.SkillManifest{
		{
			ID:             "log_anomaly_detector",
			Name:           "Log Stream Anomaly Detector",
			Category:       "observability/aiops",
			ParentCategory: "observability",
			Version:        "2.0.0",
		},
	}

	if err := cat.RegisterArmSkills("arm-weather-1", skillsArm1); err != nil {
		t.Fatalf("RegisterArmSkills failed: %v", err)
	}
	if err := cat.RegisterArmSkills("arm-aiops-1", skillsArm2); err != nil {
		t.Fatalf("RegisterArmSkills failed: %v", err)
	}

	// 1. Test Category Prefix Search
	geoSkills := cat.FindSkillsByCategory("geospatial")
	if len(geoSkills) != 2 {
		t.Errorf("expected 2 geospatial skills, got %d", len(geoSkills))
	}

	normSkills := cat.FindSkillsByCategory("geospatial/normalization")
	if len(normSkills) != 1 || normSkills[0].Manifest.ID != "geo_disambiguation" {
		t.Errorf("expected 1 normalization skill, got %v", normSkills)
	}

	// 2. Test Skill Resolution & Version Pinning
	resolved, err := cat.ResolveSkill("geo_disambiguation", "")
	if err != nil || resolved.Manifest.Version != "1.0.0" {
		t.Errorf("failed to resolve skill without pin: %v", err)
	}

	// 3. Test Hierarchical Tree Structure
	tree := cat.BuildTree()
	if tree == nil || len(tree.Children) != 2 {
		t.Fatalf("expected root to have 2 top-level category children (geospatial, observability), got %v", tree)
	}

	// 4. Test Deactivation
	cat.DeactivateArmSkills("arm-weather-1")
	_, err = cat.ResolveSkill("geo_disambiguation", "")
	if err != ErrSkillNotFound {
		t.Errorf("expected ErrSkillNotFound after arm deactivation, got %v", err)
	}

	// 5. Test Cold Boot Reload from Disk
	cat2 := NewSkillCatalog(tempDir)
	if err := cat2.LoadFromDisk(); err != nil {
		t.Fatalf("LoadFromDisk failed: %v", err)
	}
	if len(cat2.skillsByID) != 3 {
		t.Errorf("expected 3 skills reloaded from disk cache, got %d", len(cat2.skillsByID))
	}
}

func TestSkillCatalog_VersionManagement(t *testing.T) {
	cat := NewSkillCatalog("")

	// Register version 1.0.0
	_ = cat.RegisterArmSkills("arm-v1", []protocol.SkillManifest{
		{
			ID:       "summarizer",
			Name:     "Text Summarizer",
			Category: "nlp/summarization",
			Version:  "1.0.0",
		},
	})

	// Register higher version 2.1.0 from another arm
	_ = cat.RegisterArmSkills("arm-v2", []protocol.SkillManifest{
		{
			ID:       "summarizer",
			Name:     "Text Summarizer Enhanced",
			Category: "nlp/summarization",
			Version:  "2.1.0",
		},
	})

	// Unpinned resolution should return latest version (2.1.0)
	latest, err := cat.ResolveSkill("summarizer", "")
	if err != nil || latest.Manifest.Version != "2.1.0" {
		t.Errorf("expected latest version 2.1.0, got %v (err: %v)", latest, err)
	}

	// Explicitly pinned resolution
	v1, err := cat.ResolveSkill("summarizer", "1.0.0")
	if err != nil || v1.Manifest.Version != "1.0.0" {
		t.Errorf("expected pinned version 1.0.0, got %v (err: %v)", v1, err)
	}

	// Non-existent version pin
	_, err = cat.ResolveSkill("summarizer", "3.0.0")
	if err == nil {
		t.Errorf("expected error for non-existent version pin 3.0.0")
	}
}
