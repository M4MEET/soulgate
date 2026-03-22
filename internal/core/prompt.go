package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxFileChars     = 8000  // Max chars per workspace file
	maxTotalChars    = 16000 // Max total workspace files
	truncationMarker = "\n[...truncated...]"
)

// buildSystemPrompt builds a compact system prompt. Every token here is sent
// on every API call, so brevity matters. Tool descriptions are already in the
// tool schemas — don't repeat them here.
func buildSystemPrompt(workspaceRoot string, configDir string, availableTools []string, currentProvider string, currentModel string) string {
	var sb strings.Builder

	sb.WriteString(`You are SoulGate — an AI assistant with full system access.

Rules:
- Chat naturally for greetings/questions. Only use tools when asked to DO something.
- Be concise. Match the energy — short input = short reply.
- When action is needed, do it yourself. Never say "you can run..." — YOU run it.
- If something fails, fix it. If a tool is missing, install it. Ask the user only for secrets/credentials.
- Never fabricate data. Never modify internal/, cmd/, go.mod, go.sum.
- Build projects in projects/<name>/. Extend via skills/<name>/SKILL.md or .soulgate/ config.
`)

	// Model identity
	sb.WriteString(fmt.Sprintf("\nModel: %s (provider: %s)\n", currentModel, currentProvider))

	// Runtime context
	now := time.Now()
	hostname, _ := os.Hostname()
	sb.WriteString(fmt.Sprintf("\nWorkspace: %s | Host: %s | Time: %s\n",
		workspaceRoot, hostname, now.Format("2006-01-02 15:04 MST")))

	// Workspace files (SOUL.md, MEMORY.md, etc.)
	wsFiles := buildWorkspaceFilesSection(configDir)
	if wsFiles != "" {
		sb.WriteString("\n")
		sb.WriteString(wsFiles)
	}

	return sb.String()
}

// buildWorkspaceFilesSection injects workspace files into the prompt
func buildWorkspaceFilesSection(configDir string) string {
	files := []struct {
		name string
		path string
	}{
		{"SOUL.md", filepath.Join(configDir, "SOUL.md")},
		{"AGENTS.md", filepath.Join(configDir, "AGENTS.md")},
		{"MEMORY.md", filepath.Join(configDir, "MEMORY.md")},
	}

	var sb strings.Builder
	totalChars := 0

	for _, file := range files {
		content, err := readWorkspaceFile(file.path, maxFileChars)
		if err != nil {
			continue
		}
		if totalChars+len(content) > maxTotalChars {
			break
		}
		sb.WriteString(fmt.Sprintf("### %s\n\n%s\n\n", file.name, content))
		totalChars += len(content)
	}

	return sb.String()
}

// buildSkillsSection loads and injects skills into the prompt
func buildSkillsSection(workspaceRoot string) string {
	skillsDir := filepath.Join(workspaceRoot, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Skills\n\n")

	loaded := 0
	totalChars := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillFile := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		content, err := readWorkspaceFile(skillFile, 4000)
		if err != nil {
			continue
		}
		if totalChars+len(content) > 8000 {
			break
		}
		sb.WriteString(fmt.Sprintf("### %s\n\n%s\n\n", entry.Name(), content))
		totalChars += len(content)
		loaded++
	}

	if loaded == 0 {
		return ""
	}
	return sb.String()
}

// readWorkspaceFile reads a workspace file with size limits
func readWorkspaceFile(path string, maxChars int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := string(data)
	if len(content) > maxChars {
		content = content[:maxChars] + truncationMarker
	}
	return content, nil
}

// isCoreTool checks if a tool is a core tool
func isCoreTool(name string) bool {
	coreTools := map[string]bool{
		"files_read": true, "files_write": true, "files_list": true, "files_delete": true,
		"exec_command": true, "net_request": true,
		"memory_write": true, "memory_search": true, "memory_get": true,
		"agent_create": true, "agent_list": true, "agent_stop": true,
		"switch_model": true,
		"web_search":   true, "web_fetch": true,
		"process_start": true, "process_list": true, "process_poll": true,
		"process_log": true, "process_write": true, "process_kill": true,
		"pdf_read": true,
		"cron_add": true, "cron_list": true, "cron_remove": true,
		"cron_pause": true, "cron_resume": true,
		"llm_task": true, "apply_patch": true,
		"soulgate_introspect": true, "soulgate_configure": true,
	}
	return coreTools[name]
}
