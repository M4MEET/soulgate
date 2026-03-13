package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillLoader(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "skills-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test skill
	skillDir := filepath.Join(tmpDir, "code_review")
	err = os.MkdirAll(skillDir, 0755)
	require.NoError(t, err)

	skillContent := `# Code Review

Expert at reviewing code for bugs and improvements.

When reviewing code:
- Check for bugs and edge cases
- Suggest improvements
- Verify best practices
`

	err = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644)
	require.NoError(t, err)

	// Test loading
	loader := NewLoader(tmpDir)

	// Test Load single skill
	skill, err := loader.Load("code_review")
	require.NoError(t, err)
	assert.Equal(t, "code_review", skill.ID)
	assert.Equal(t, "Code Review", skill.Name)
	assert.Contains(t, skill.Description, "Expert at reviewing")
	assert.Contains(t, skill.Content, "When reviewing code")

	// Test LoadAll
	skills, err := loader.LoadAll()
	require.NoError(t, err)
	assert.Len(t, skills, 1)
	assert.Equal(t, "code_review", skills[0].ID)

	// Test ListSkills
	skillIDs, err := loader.ListSkills()
	require.NoError(t, err)
	assert.Contains(t, skillIDs, "code_review")

	// Test LoadByIDs
	skills, err = loader.LoadByIDs([]string{"code_review"})
	require.NoError(t, err)
	assert.Len(t, skills, 1)
}

func TestBuildSkillContext(t *testing.T) {
	skills := []Skill{
		{
			ID:          "test",
			Name:        "Test Skill",
			Description: "A test skill",
			Content:     "Test content",
		},
	}

	context := BuildSkillContext(skills)
	assert.Contains(t, context, "Available Skills")
	assert.Contains(t, context, "Test Skill")
	assert.Contains(t, context, "Test content")
}

func TestLoadNonExistentSkill(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skills-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	loader := NewLoader(tmpDir)
	_, err = loader.Load("nonexistent")
	assert.Error(t, err)
}

func TestValidateSkill(t *testing.T) {
	// Valid skill
	skill := &Skill{
		ID:      "test",
		Content: "Some content",
	}
	assert.NoError(t, ValidateSkill(skill))

	// Missing ID
	skill = &Skill{
		Content: "Some content",
	}
	assert.Error(t, ValidateSkill(skill))

	// Empty content
	skill = &Skill{
		ID:      "test",
		Content: "",
	}
	assert.Error(t, ValidateSkill(skill))
}

func TestParseSkillHeader(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantName string
		wantDesc string
	}{
		{
			name: "with skill prefix",
			content: `# Skill: Code Review

Expert at reviewing code.

More content here.`,
			wantName: "Code Review",
			wantDesc: "Expert at reviewing code.",
		},
		{
			name: "without prefix",
			content: `# Debugging

Find and fix bugs.

Instructions:
- Check logs
`,
			wantName: "Debugging",
			wantDesc: "Find and fix bugs.",
		},
		{
			name: "no header",
			content: `Some content without header

More content.`,
			wantName: "Unnamed Skill",
			wantDesc: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, desc := parseSkillHeader(tt.content)
			assert.Equal(t, tt.wantName, name)
			if tt.wantDesc != "" {
				assert.Contains(t, desc, tt.wantDesc)
			}
		})
	}
}
