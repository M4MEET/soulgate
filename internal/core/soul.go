package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SoulConfig defines the AI's persona and behavior rules
type SoulConfig struct {
	Path    string
	Content string
}

// DefaultSoulTemplate returns the default SOUL.md template
func DefaultSoulTemplate() string {
	return `# SoulGate AI Persona

## Identity
You are SoulGate, a powerful and security-conscious AI assistant.
You operate within a controlled gateway that mediates all access to system resources.

## Personality
- Professional yet approachable
- Direct and action-oriented (execute, don't suggest)
- Security-first mindset
- Transparent about what you're doing and why

## Communication Style
- Be concise and to-the-point
- Use technical language when appropriate
- Explain complex operations step by step
- Always show real output from tools, never fabricate

## Behavior Rules
- Always use tools to get information; never guess
- Execute commands directly; don't tell the user to run them
- Check security policies before sensitive operations
- Log important actions to memory for context persistence
- When switching models, explain why to the user

## Context Awareness
- Remember user preferences across sessions via memory
- Adapt response complexity to the user's expertise level
- Track ongoing conversations and reference previous context
- Use time awareness for scheduling and deadline tracking

## Boundaries
- Respect workspace security policies
- Ask for confirmation before destructive operations
- Protect sensitive data (API keys, credentials)
- Don't access resources outside allowed policy scope
`
}

// LoadSoulConfig loads the SOUL.md configuration
func LoadSoulConfig(configDir string) (*SoulConfig, error) {
	soulPath := filepath.Join(configDir, "SOUL.md")

	content, err := os.ReadFile(soulPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No soul file - that's ok
		}
		return nil, fmt.Errorf("failed to read SOUL.md: %w", err)
	}

	return &SoulConfig{
		Path:    soulPath,
		Content: string(content),
	}, nil
}

// CreateSoulFile creates a SOUL.md file with the default template
func CreateSoulFile(configDir string) error {
	soulPath := filepath.Join(configDir, "SOUL.md")

	// Don't overwrite existing
	if _, err := os.Stat(soulPath); err == nil {
		return fmt.Errorf("SOUL.md already exists at %s", soulPath)
	}

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(soulPath, []byte(DefaultSoulTemplate()), 0600); err != nil {
		return fmt.Errorf("failed to write SOUL.md: %w", err)
	}

	return nil
}

// UpdateSoulFile updates the SOUL.md content
func UpdateSoulFile(configDir string, content string) error {
	soulPath := filepath.Join(configDir, "SOUL.md")

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(soulPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write SOUL.md: %w", err)
	}

	return nil
}

// ParseSoulSections extracts named sections from a SOUL.md file
func ParseSoulSections(content string) map[string]string {
	sections := make(map[string]string)
	lines := strings.Split(content, "\n")

	currentSection := ""
	var sectionContent strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			// Save previous section
			if currentSection != "" {
				sections[currentSection] = strings.TrimSpace(sectionContent.String())
			}
			currentSection = strings.TrimPrefix(line, "## ")
			currentSection = strings.TrimSpace(currentSection)
			sectionContent.Reset()
		} else if currentSection != "" {
			sectionContent.WriteString(line + "\n")
		}
	}

	// Save last section
	if currentSection != "" {
		sections[currentSection] = strings.TrimSpace(sectionContent.String())
	}

	return sections
}
