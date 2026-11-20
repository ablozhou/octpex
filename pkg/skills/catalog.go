package skills

import (
	"encoding/json"
	"errors"
	"fmt"
	"octpex/pkg/protocol"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrSkillNotFound = errors.New("skills: skill not found in catalog")
	ErrNoVersionMatch = errors.New("skills: no matching skill version found")
)

// SkillEntry represents a registered skill instance with its host arm and version metadata.
type SkillEntry struct {
	Manifest       protocol.SkillManifest `json:"manifest"`
	ArmID          string                 `json:"arm_id"`
	RegisteredAt   time.Time              `json:"registered_at"`
	LastRefreshedAt time.Time             `json:"last_refreshed_at"`
	IsActive       bool                   `json:"is_active"`
}

// SkillCatalog manages hierarchical routing, version negotiation, and disk persistence for Octo-Brain skills.
type SkillCatalog struct {
	mu          sync.RWMutex
	skillsByID  map[string][]*SkillEntry // skill_id -> list of versions/providers
	storageDir  string                   // Path for local disk cache
}

// NewSkillCatalog initializes an empty SkillCatalog.
func NewSkillCatalog(storageDir string) *SkillCatalog {
	if storageDir != "" {
		_ = os.MkdirAll(storageDir, 0755)
	}
	return &SkillCatalog{
		skillsByID: make(map[string][]*SkillEntry),
		storageDir: storageDir,
	}
}

// RegisterArmSkills ingests all skills declared by an arm, updating the routing index and persisting to disk.
func (c *SkillCatalog) RegisterArmSkills(armID string, skills []protocol.SkillManifest) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for _, skill := range skills {
		if skill.ID == "" {
			continue
		}
		if skill.Version == "" {
			skill.Version = "1.0.0"
		}
		if skill.Category == "" {
			if skill.ParentCategory != "" {
				skill.Category = skill.ParentCategory
			} else {
				skill.Category = "general"
			}
		}

		entry := &SkillEntry{
			Manifest:        skill,
			ArmID:           armID,
			RegisteredAt:    now,
			LastRefreshedAt: now,
			IsActive:        true,
		}

		entries := c.skillsByID[skill.ID]
		updated := false
		for i, e := range entries {
			if e.ArmID == armID && e.Manifest.Version == skill.Version {
				entries[i] = entry
				updated = true
				break
			}
		}
		if !updated {
			entries = append(entries, entry)
		}
		c.skillsByID[skill.ID] = entries

		// Asynchronously or best-effort persist skill definition to disk
		if c.storageDir != "" {
			_ = c.persistSkillToDisk(entry)
		}
	}

	return nil
}

// DeactivateArmSkills marks all skills belonging to an arm as inactive (e.g. on lease expiry).
func (c *SkillCatalog) DeactivateArmSkills(armID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, entries := range c.skillsByID {
		for _, e := range entries {
			if e.ArmID == armID {
				e.IsActive = false
			}
		}
	}
}

// ResolveSkill finds the active skill entry by ID, supporting version pinning or latest semver fallback.
func (c *SkillCatalog) ResolveSkill(skillID string, versionPin string) (*SkillEntry, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entries, exists := c.skillsByID[skillID]
	if !exists || len(entries) == 0 {
		return nil, ErrSkillNotFound
	}

	// Filter active entries
	var active []*SkillEntry
	for _, e := range entries {
		if e.IsActive {
			active = append(active, e)
		}
	}
	if len(active) == 0 {
		return nil, ErrSkillNotFound
	}

	// If explicit version requested
	if versionPin != "" {
		for _, e := range active {
			if e.Manifest.Version == versionPin {
				return e, nil
			}
		}
		return nil, fmt.Errorf("%w: requested %s, available versions for %s", ErrNoVersionMatch, versionPin, skillID)
	}

	// Default: select highest version
	sort.Slice(active, func(i, j int) bool {
		return active[i].Manifest.Version > active[j].Manifest.Version
	})

	return active[0], nil
}

// FindSkillsByCategory returns all active skills matching a category prefix (e.g. "geospatial" or "geospatial/normalization").
func (c *SkillCatalog) FindSkillsByCategory(categoryPrefix string) []*SkillEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	prefix := strings.Trim(categoryPrefix, "/")
	var results []*SkillEntry

	for _, entries := range c.skillsByID {
		for _, e := range entries {
			if !e.IsActive {
				continue
			}
			matched := false
			allCats := append([]string{e.Manifest.Category}, e.Manifest.Categories...)
			for _, catRaw := range allCats {
				cat := strings.Trim(catRaw, "/")
				if cat == prefix || strings.HasPrefix(cat, prefix+"/") || prefix == "" {
					matched = true
					break
				}
			}
			if matched {
				results = append(results, e)
			}
		}
	}

	return results
}

// BuildTree constructs a hierarchical category tree containing all active skills.
func (c *SkillCatalog) BuildTree() *protocol.SkillTreeNode {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Convert active skills into synthetic ArmManifest structures for protocol.BuildSkillTree
	armMap := make(map[string][]protocol.SkillManifest)
	for _, entries := range c.skillsByID {
		for _, e := range entries {
			if e.IsActive {
				armMap[e.ArmID] = append(armMap[e.ArmID], e.Manifest)
			}
		}
	}

	arms := make([]*protocol.ArmManifest, 0, len(armMap))
	for armID, sList := range armMap {
		arms = append(arms, &protocol.ArmManifest{
			ArmID:  armID,
			Skills: sList,
		})
	}

	return protocol.BuildSkillTree(arms)
}

// persistSkillToDisk writes the skill definition as a structured JSON file organized by category.
func (c *SkillCatalog) persistSkillToDisk(entry *SkillEntry) error {
	catDir := filepath.Join(c.storageDir, filepath.FromSlash(entry.Manifest.Category))
	if err := os.MkdirAll(catDir, 0755); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s_v%s.json", entry.Manifest.ID, entry.Manifest.Version)
	filePath := filepath.Join(catDir, filename)

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

// LoadFromDisk reloads previously persisted skill definitions into the catalog on cold boot.
func (c *SkillCatalog) LoadFromDisk() error {
	if c.storageDir == "" {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return filepath.Walk(c.storageDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var entry SkillEntry
		if err := json.Unmarshal(data, &entry); err == nil && entry.Manifest.ID != "" {
			entry.IsActive = false // Loaded from cold storage, awaits active arm heartbeat
			c.skillsByID[entry.Manifest.ID] = append(c.skillsByID[entry.Manifest.ID], &entry)
		}
		return nil
	})
}
