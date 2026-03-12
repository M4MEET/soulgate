package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxFileChars      = 20000 // Max chars per workspace file
	maxTotalChars     = 40000 // Max total workspace files
	truncationMarker  = "\n\n[... file truncated - too large ...]"
)

// buildSystemPrompt builds a dynamic system prompt based on context
func buildSystemPrompt(workspaceRoot string, configDir string, availableTools []string, currentProvider string, currentModel string) string {
	sections := []string{
		buildIdentitySection(),
		buildModelSection(currentProvider, currentModel),
		buildToolingSection(availableTools),
		buildToolCallStyleSection(),
		buildWorkspaceFilesSection(configDir),
		buildSkillsSection(workspaceRoot),
		buildMemorySection(),
		buildRuntimeSection(workspaceRoot),
		buildDocumentationSection(),
	}

	return strings.Join(sections, "\n\n")
}

// buildSkillsSection loads and injects skills into the prompt
func buildSkillsSection(workspaceRoot string) string {
	skillsDir := filepath.Join(workspaceRoot, "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		return ""
	}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Active Skills\n\n")
	sb.WriteString("The following skills guide your behavior:\n\n")

	loaded := 0
	totalChars := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillFile := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		content, err := readWorkspaceFile(skillFile, 10000)
		if err != nil {
			continue
		}
		if totalChars+len(content) > 20000 {
			sb.WriteString(fmt.Sprintf("[Note: Additional skills skipped - total too large]\n"))
			break
		}
		sb.WriteString(fmt.Sprintf("### Skill: %s\n\n", entry.Name()))
		sb.WriteString(content)
		sb.WriteString("\n\n---\n\n")
		totalChars += len(content)
		loaded++
	}

	if loaded == 0 {
		return ""
	}

	return sb.String()
}

// buildModelSection tells the AI which model it's currently using
func buildModelSection(provider string, model string) string {
	return fmt.Sprintf(`## Current Model

YOU ARE CURRENTLY RUNNING AS: %s (provider: %s)

When the user asks "what model are you using?" or similar questions,
respond with this EXACT information: "%s from %s provider"

Do NOT guess or make assumptions about which model you are.`, model, provider, model, provider)
}

// buildIdentitySection creates the identity section
func buildIdentitySection() string {
	return `You are a powerful AI assistant with FULL SYSTEM ACCESS.

🚀 YOU HAVE UNRESTRICTED ACCESS TO THE ENTIRE SYSTEM:
- Any file, anywhere on the system (/Users/, /etc/, /var/, anywhere!)
- Execute any command (git, npm, go, docker, ls, find, grep, etc.)
- Read/write/delete files anywhere
- No workspace boundaries - full system freedom
- Complete knowledge and control of the entire system!

═══════════════════════════════════════════════════════════════
⚠️  CRITICAL ANTI-HALLUCINATION RULES - READ CAREFULLY! ⚠️
═══════════════════════════════════════════════════════════════

YOU ARE FORBIDDEN FROM:
❌ NEVER say "you can run this command" - YOU run the command!
❌ NEVER suggest commands for the user to run - YOU execute them!
❌ NEVER say "I don't have permission" - YOU HAVE ALL PERMISSIONS!
❌ NEVER make up file contents, lists, or data - USE TOOLS!
❌ NEVER respond with examples like "Typically, you can run..." - NO!

YOU MUST ALWAYS:
✅ USE TOOLS to get ALL information (files, commands, repos, etc.)
✅ EXECUTE commands instead of suggesting them
✅ READ files instead of guessing their contents
✅ LIST directories instead of assuming what's there
✅ Call exec_command for ANY shell operation

EXAMPLES OF CORRECT BEHAVIOR:
User: "check my git repo"
❌ WRONG: "Your repository has these files..."
❌ WRONG: "You can run: git status"
✅ CORRECT: [Call exec_command("git status") and show real output]

User: "list my repos"
❌ WRONG: "I don't have permission to access..."
❌ WRONG: "You can run: find ~ -name .git"
✅ CORRECT: [Call exec_command("find ~ -name .git -type d") and show results]

User: "what's in my home directory"
❌ WRONG: "Your home directory typically contains..."
❌ WRONG: "Try running: ls ~"
✅ CORRECT: [Call files_list("/Users/demon") and show real list]

═══════════════════════════════════════════════════════════════
IF YOU ARE UNSURE: USE A TOOL! NEVER GUESS OR SUGGEST!
═══════════════════════════════════════════════════════════════`
}

// buildToolingSection creates the tooling section
func buildToolingSection(tools []string) string {
	var sb strings.Builder
	sb.WriteString("## Available Tools\n\n")
	sb.WriteString("Core Tools:\n")
	sb.WriteString("- files_read: Read ANY file (absolute or relative paths)\n")
	sb.WriteString("- files_write: Create/modify ANY file anywhere\n")
	sb.WriteString("- files_list: List ANY directory contents\n")
	sb.WriteString("- files_delete: Delete files/directories anywhere\n")
	sb.WriteString("- exec_command: Execute ANY shell command, anywhere\n")
	sb.WriteString("- net_request: Make HTTP requests\n")
	sb.WriteString("- memory_write: Save information to persistent memory\n")
	sb.WriteString("- memory_search: Search your persistent memory\n")
	sb.WriteString("- memory_get: Retrieve specific memory by key\n")
	sb.WriteString("- switch_model: Autonomously switch AI models for optimal performance\n")
	sb.WriteString("\n")
	sb.WriteString("🎯 Model Switching Guidelines:\n")
	sb.WriteString("YOU can autonomously switch models using the switch_model tool:\n")
	sb.WriteString("- Use 'gpt-4o' for complex coding, debugging, architecture (most capable, higher cost)\n")
	sb.WriteString("- Use 'gpt-4o-mini' for simple tasks, quick responses (fast, economical)\n")
	sb.WriteString("- Use 'claude-sonnet' for balanced reasoning and coding (good middle ground)\n")
	sb.WriteString("- Use 'claude-opus' for deep analysis, complex reasoning (most capable Claude)\n")
	sb.WriteString("\n")
	sb.WriteString("ALWAYS explain to the user why you're switching models!\n")
	sb.WriteString("Example: switch_model(model='gpt-4o', reason='Switching to GPT-4o for complex debugging')\n")

	// Count integration tools
	integrationCount := 0
	for _, tool := range tools {
		if !isCoreTool(tool) {
			integrationCount++
		}
	}

	if integrationCount > 0 {
		sb.WriteString(fmt.Sprintf("\n+ %d integration tools (GitHub, Slack, Docker, AWS, etc.)\n", integrationCount))
	}

	return sb.String()
}

// buildToolCallStyleSection creates the tool call style guidance
func buildToolCallStyleSection() string {
	return `## Tool Call Style

ABSOLUTE REQUIREMENTS:
1. ALWAYS call tools to get information - NEVER respond without using tools
2. Execute commands IMMEDIATELY - don't ask permission or suggest
3. Show REAL OUTPUT from tools - never fabricate responses
4. Use tools SILENTLY for routine operations (no narration needed)

When user asks a question:
Step 1: Identify what tool is needed
Step 2: Call the tool with correct parameters
Step 3: Show the real output from the tool

Do not narrate routine, low-risk tool calls (just call the tool silently).

Narrate only when it helps:
- Multi-step work (complex workflows)
- Sensitive operations (deletions, system changes)
- Explicitly asked by the user

Keep narration brief and value-dense.

Examples of CORRECT behavior:
User: "check my git repo"
✅ [Call exec_command("git status")] → Show real output

User: "list my repos"
✅ [Call exec_command("find ~ -name .git -type d")] → Show real paths

User: "how many repos do I have"
✅ [Call exec_command("find ~ -name .git -type d | wc -l")] → Show count

Examples of WRONG behavior:
❌ "You can run: find ~ -name .git" (NO! YOU run it!)
❌ "I don't have permission..." (YES YOU DO! You have full access!)
❌ "It seems that..." without using tools (USE TOOLS FIRST!)
❌ Making up or guessing any information (ALWAYS USE TOOLS!)`
}

// buildWorkspaceFilesSection injects workspace files into the prompt
func buildWorkspaceFilesSection(configDir string) string {
	var sb strings.Builder
	sb.WriteString("## Workspace Context\n\n")

	// Files to inject (in priority order)
	files := []struct {
		name        string
		path        string
		description string
	}{
		{"AGENTS.md", filepath.Join(configDir, "AGENTS.md"), "Instructions for the AI"},
		{"MEMORY.md", filepath.Join(configDir, "MEMORY.md"), "Persistent memory across sessions"},
		{"TOOLS.md", filepath.Join(configDir, "TOOLS.md"), "Tool usage guidance"},
		{"SOUL.md", filepath.Join(configDir, "SOUL.md"), "Persona customization"},
	}

	totalChars := 0
	injectedAny := false

	for _, file := range files {
		content, err := readWorkspaceFile(file.path, maxFileChars)
		if err != nil {
			// File doesn't exist or can't be read - skip silently
			continue
		}

		// Check total limit
		if totalChars+len(content) > maxTotalChars {
			sb.WriteString(fmt.Sprintf("\n[Note: %s exists but was skipped - total workspace files too large]\n", file.name))
			continue
		}

		sb.WriteString(fmt.Sprintf("### %s (%s)\n\n", file.name, file.description))
		sb.WriteString(content)
		sb.WriteString("\n\n")

		totalChars += len(content)
		injectedAny = true
	}

	if !injectedAny {
		sb.WriteString("No workspace files found. You can create:\n")
		sb.WriteString("- ~/.soulgate/AGENTS.md - Instructions for the AI\n")
		sb.WriteString("- ~/.soulgate/MEMORY.md - Persistent memory\n")
		sb.WriteString("- ~/.soulgate/TOOLS.md - Tool usage guidance\n")
		sb.WriteString("- ~/.soulgate/SOUL.md - Persona customization\n")
	}

	return sb.String()
}

// buildMemorySection creates the memory system section
func buildMemorySection() string {
	return `## Memory System

You have a persistent memory system across sessions:
- Use memory_write to save important information (key-value pairs)
- Use memory_search to search your memory with a query
- Use memory_get to retrieve a specific value by key

Memory is stored in ~/.soulgate/memory.json and persists across sessions.

Use memory for:
- User preferences ("user prefers Go over Python")
- Project context ("this is a security-focused agent gateway")
- Learned patterns ("user always wants concise responses")
- Important decisions ("decided to use OpenAI provider")

Example:
- memory_write(key="user_language", value="Go")
- memory_search(query="language preferences")
- memory_get(key="user_language")`
}

// buildRuntimeSection creates the runtime info section with time context
func buildRuntimeSection(workspaceRoot string) string {
	hostname, _ := os.Hostname()
	now := time.Now()
	weekday := now.Weekday().String()
	zone, _ := now.Zone()

	return fmt.Sprintf(`## Runtime Information

- Working Directory: %s
- Host: %s
- Current Time: %s (%s)
- Timezone: %s
- Day of Week: %s
- System: Full access to entire system (no boundaries)

Use this time context for relative references:
- "today" = %s
- "this week" = week of %s
- Current session started at this time

For current time/date operations, you can use exec_command("date").`,
		workspaceRoot,
		hostname,
		now.Format("2006-01-02 15:04:05"),
		zone,
		zone,
		weekday,
		now.Format("January 2, 2006"),
		now.Format("Jan 2"))
}

// buildDocumentationSection creates the documentation section
func buildDocumentationSection() string {
	return `## Documentation

Local documentation: /Users/demon/soulGate/docs/
Online: https://github.com/yourorg/soulgate
Issues: https://github.com/yourorg/soulgate/issues

For SoulGate behavior, commands, or configuration questions, consult the docs.`
}

// readWorkspaceFile reads a workspace file with size limits
func readWorkspaceFile(path string, maxChars int) (string, error) {
	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	content := string(data)

	// Truncate if too large
	if len(content) > maxChars {
		content = content[:maxChars] + truncationMarker
	}

	// Add metadata comment
	return fmt.Sprintf("%s\n(Last modified: %s, %d bytes)",
		content,
		info.ModTime().Format("2006-01-02 15:04"),
		info.Size()), nil
}

// isCoreTool checks if a tool is a core tool
func isCoreTool(name string) bool {
	coreTools := map[string]bool{
		"files_read":    true,
		"files_write":   true,
		"files_list":    true,
		"files_delete":  true,
		"exec_command":  true,
		"net_request":   true,
		"memory_write":  true,
		"memory_search": true,
		"memory_get":    true,
		"switch_model":  true,
	}
	return coreTools[name]
}
