package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Skill represents a loaded skill
type Skill struct {
	ID          string            // Skill directory name
	Name        string            // Display name
	Description string            // Short description
	Content     string            // Full markdown content
	Metadata    map[string]string // Optional metadata
}

// Loader loads skills from markdown files
type Loader struct {
	skillsDir string
}

// NewLoader creates a new skill loader
func NewLoader(skillsDir string) *Loader {
	return &Loader{
		skillsDir: skillsDir,
	}
}

// LoadAll loads all skills from the skills directory
func (l *Loader) LoadAll() ([]Skill, error) {
	// Check if skills directory exists
	if _, err := os.Stat(l.skillsDir); os.IsNotExist(err) {
		return []Skill{}, nil // No skills directory, return empty
	}

	entries, err := os.ReadDir(l.skillsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read skills directory: %w", err)
	}

	var skills []Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillID := entry.Name()
		skill, err := l.Load(skillID)
		if err != nil {
			// Log warning but continue
			fmt.Printf("Warning: failed to load skill %s: %v\n", skillID, err)
			continue
		}

		skills = append(skills, *skill)
	}

	return skills, nil
}

// Load loads a single skill by ID
func (l *Loader) Load(skillID string) (*Skill, error) {
	skillDir := filepath.Join(l.skillsDir, skillID)

	// Check if skill directory exists
	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("skill not found: %s", skillID)
	}

	// Look for SKILL.md file
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(skillFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("SKILL.md not found in %s", skillID)
	}

	// Read skill content
	content, err := os.ReadFile(skillFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read SKILL.md: %w", err)
	}

	// Parse skill
	skill := &Skill{
		ID:       skillID,
		Content:  string(content),
		Metadata: make(map[string]string),
	}

	// Extract metadata from frontmatter or first lines
	skill.Name, skill.Description = parseSkillHeader(string(content))

	return skill, nil
}

// LoadByIDs loads multiple skills by their IDs
func (l *Loader) LoadByIDs(skillIDs []string) ([]Skill, error) {
	var skills []Skill

	for _, id := range skillIDs {
		skill, err := l.Load(id)
		if err != nil {
			return nil, fmt.Errorf("failed to load skill %s: %w", id, err)
		}
		skills = append(skills, *skill)
	}

	return skills, nil
}

// ListSkills lists all available skill IDs
func (l *Loader) ListSkills() ([]string, error) {
	if _, err := os.Stat(l.skillsDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(l.skillsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read skills directory: %w", err)
	}

	var skillIDs []string
	for _, entry := range entries {
		if entry.IsDir() {
			// Check if SKILL.md exists
			skillFile := filepath.Join(l.skillsDir, entry.Name(), "SKILL.md")
			if _, err := os.Stat(skillFile); err == nil {
				skillIDs = append(skillIDs, entry.Name())
			}
		}
	}

	return skillIDs, nil
}

// skillScaffoldTemplate is the SKILL.md content written by CreateSkill.
// The caller substitutes %s for the skill ID.
const skillScaffoldTemplate = `# Skill: %s

A brief one-sentence description of what this skill does.

## Behavior

Describe how the agent should behave when this skill is active. Be specific about
decision-making criteria, tone, and constraints.

## Tools

List the tools or resources this skill is expected to use:

- files.read  - reading relevant workspace files
- (add others as needed)

## Examples

### Example 1

**Input:** A representative user request.

**Expected behavior:** What the agent should do in response.

### Example 2

**Input:** Another representative request, ideally an edge case.

**Expected behavior:** How the agent handles it correctly.

## Notes

Any additional guidance, limitations, or context the agent should be aware of.
`

// CreateSkill creates a scaffold directory and SKILL.md for a new skill.
// It returns the populated Skill so callers can inspect it without a second Load call.
func (l *Loader) CreateSkill(skillID string) (*Skill, error) {
	if skillID == "" {
		return nil, fmt.Errorf("skill ID cannot be empty")
	}

	skillDir := filepath.Join(l.skillsDir, skillID)

	// Refuse to overwrite an existing skill to avoid accidental data loss.
	if _, err := os.Stat(skillDir); err == nil {
		return nil, fmt.Errorf("skill already exists: %s", skillID)
	}

	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create skill directory: %w", err)
	}

	content := fmt.Sprintf(skillScaffoldTemplate, skillID)
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write SKILL.md: %w", err)
	}

	skill := &Skill{
		ID:       skillID,
		Content:  content,
		Metadata: make(map[string]string),
	}
	skill.Name, skill.Description = parseSkillHeader(content)

	return skill, nil
}

// BuildSkillContext builds a context string from skills for injection into prompts
func BuildSkillContext(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("# Available Skills\n\n")
	builder.WriteString("You have access to the following skills that guide your behavior:\n\n")

	for _, skill := range skills {
		builder.WriteString(fmt.Sprintf("## Skill: %s\n\n", skill.Name))
		if skill.Description != "" {
			builder.WriteString(fmt.Sprintf("%s\n\n", skill.Description))
		}
		builder.WriteString(skill.Content)
		builder.WriteString("\n\n---\n\n")
	}

	return builder.String()
}

// parseSkillHeader extracts name and description from markdown content
func parseSkillHeader(content string) (name, description string) {
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Look for # Skill: Name or # Name
		if strings.HasPrefix(line, "# ") {
			name = strings.TrimPrefix(line, "# ")
			name = strings.TrimPrefix(name, "Skill: ")
			name = strings.TrimSpace(name)

			// Next non-empty line might be description
			for j := i + 1; j < len(lines) && j < i+5; j++ {
				desc := strings.TrimSpace(lines[j])
				if desc != "" && !strings.HasPrefix(desc, "#") {
					description = desc
					break
				}
			}
			break
		}
	}

	// Default name if not found
	if name == "" {
		name = "Unnamed Skill"
	}

	return name, description
}

// ValidateSkill validates a skill structure
func ValidateSkill(skill *Skill) error {
	if skill.ID == "" {
		return fmt.Errorf("skill ID is required")
	}
	if skill.Content == "" {
		return fmt.Errorf("skill content is empty")
	}
	if len(skill.Content) > 100000 {
		return fmt.Errorf("skill content too large (max 100KB)")
	}
	return nil
}
