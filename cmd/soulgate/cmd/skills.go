package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/skills"
	"github.com/spf13/cobra"
)

// errWorkspaceHint is appended to workspace-load failures to guide the user.
const errWorkspaceHint = `
Run 'soulgate init' to initialize a workspace in the current directory.`

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage agent skills",
	Long: `Manage markdown-based skills that guide agent behavior.

Skills are stored in workspace/skills/<skill-id>/SKILL.md and can be
loaded by agents to provide specialized knowledge and behavior patterns.

Example:
  soulgate skills list              # List all available skills
  soulgate skills show code_review  # Show skill content
  soulgate skills validate          # Validate all skills`,
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available skills",
	RunE:  runSkillsList,
}

var skillsShowCmd = &cobra.Command{
	Use:   "show <skill-id>",
	Short: "Show skill content",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsShow,
}

var skillsValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate all skills",
	RunE:  runSkillsValidate,
}

var skillsCreateCmd = &cobra.Command{
	Use:   "create <skill-id>",
	Short: "Create a new skill scaffold",
	Long: `Create a new skill directory and starter SKILL.md file.

The skill ID must be a simple identifier (letters, numbers, underscores, hyphens).
The scaffold is created at skills/<skill-id>/SKILL.md inside the workspace.

Example:
  soulgate skills create code_review
  soulgate skills create sql_helper`,
	Args: cobra.ExactArgs(1),
	RunE: runSkillsCreate,
}

var skillsConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Show or change skill configuration",
	Long: `Show which skills are enabled and enable or disable individual skills.

Subcommands:
  list              Show all skills and their enabled/disabled state
  enable <id>       Enable a skill
  disable <id>      Disable a skill

Example:
  soulgate skills configure list
  soulgate skills configure enable code_review
  soulgate skills configure disable sql_helper`,
}

var skillsConfigureListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show enabled/disabled state of all skills",
	RunE:  runSkillsConfigureList,
}

var skillsConfigureEnableCmd = &cobra.Command{
	Use:   "enable <skill-id>",
	Short: "Enable a skill",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsConfigureEnable,
}

var skillsConfigureDisableCmd = &cobra.Command{
	Use:   "disable <skill-id>",
	Short: "Disable a skill",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsConfigureDisable,
}

var (
	skillsWorkspaceDir string
)

func init() {
	rootCmd.AddCommand(skillsCmd)
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsShowCmd)
	skillsCmd.AddCommand(skillsValidateCmd)
	skillsCmd.AddCommand(skillsCreateCmd)
	skillsCmd.AddCommand(skillsConfigureCmd)

	skillsConfigureCmd.AddCommand(skillsConfigureListCmd)
	skillsConfigureCmd.AddCommand(skillsConfigureEnableCmd)
	skillsConfigureCmd.AddCommand(skillsConfigureDisableCmd)

	skillsCmd.PersistentFlags().StringVar(&skillsWorkspaceDir, "workspace", ".", "Workspace directory")
}

func runSkillsList(cmd *cobra.Command, args []string) error {
	workspace, err := config.LoadWorkspaceFromPath(skillsWorkspaceDir)
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w", err)
	}

	skillsDir := filepath.Join(workspace.Root, "skills")
	loader := skills.NewLoader(skillsDir)

	skillIDs, err := loader.ListSkills()
	if err != nil {
		return fmt.Errorf("failed to list skills: %w", err)
	}

	if len(skillIDs) == 0 {
		fmt.Println("No skills found")
		fmt.Printf("Skills directory: %s\n", skillsDir)
		fmt.Println("\nCreate skills by adding directories with SKILL.md files:")
		fmt.Println("  mkdir -p workspace/skills/my_skill")
		fmt.Println("  vim workspace/skills/my_skill/SKILL.md")
		return nil
	}

	fmt.Printf("Available skills (%d):\n\n", len(skillIDs))

	// Load all skills to show details
	loadedSkills, err := loader.LoadByIDs(skillIDs)
	if err != nil {
		return fmt.Errorf("failed to load skills: %w", err)
	}

	for _, skill := range loadedSkills {
		fmt.Printf("  • %s\n", skill.ID)
		fmt.Printf("    Name: %s\n", skill.Name)
		if skill.Description != "" {
			// Truncate long descriptions
			desc := skill.Description
			if len(desc) > 80 {
				desc = desc[:77] + "..."
			}
			fmt.Printf("    Description: %s\n", desc)
		}
		fmt.Println()
	}

	fmt.Printf("Skills directory: %s\n", skillsDir)
	return nil
}

func runSkillsShow(cmd *cobra.Command, args []string) error {
	skillID := args[0]

	workspace, err := config.LoadWorkspaceFromPath(skillsWorkspaceDir)
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w", err)
	}

	skillsDir := filepath.Join(workspace.Root, "skills")
	loader := skills.NewLoader(skillsDir)

	skill, err := loader.Load(skillID)
	if err != nil {
		return fmt.Errorf("failed to load skill: %w", err)
	}

	// Display skill details
	fmt.Printf("Skill: %s\n", skill.ID)
	fmt.Printf("Name: %s\n", skill.Name)
	if skill.Description != "" {
		fmt.Printf("Description: %s\n", skill.Description)
	}
	fmt.Println("\n" + strings.Repeat("─", 60))
	fmt.Println()
	fmt.Println(skill.Content)

	return nil
}

func runSkillsValidate(cmd *cobra.Command, args []string) error {
	workspace, err := config.LoadWorkspaceFromPath(skillsWorkspaceDir)
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w", err)
	}

	skillsDir := filepath.Join(workspace.Root, "skills")
	loader := skills.NewLoader(skillsDir)

	skillIDs, err := loader.ListSkills()
	if err != nil {
		return fmt.Errorf("failed to list skills: %w", err)
	}

	if len(skillIDs) == 0 {
		fmt.Println("No skills found to validate")
		return nil
	}

	fmt.Printf("Validating %d skills...\n\n", len(skillIDs))

	validCount := 0
	invalidCount := 0

	for _, skillID := range skillIDs {
		skill, err := loader.Load(skillID)
		if err != nil {
			fmt.Printf("  FAIL  %s: %v\n", skillID, err)
			invalidCount++
			continue
		}

		if err := skills.ValidateSkill(skill); err != nil {
			fmt.Printf("  FAIL  %s: %v\n", skillID, err)
			invalidCount++
			continue
		}

		fmt.Printf("  OK    %s (%d bytes)\n", skillID, len(skill.Content))
		validCount++
	}

	fmt.Println()
	fmt.Printf("Valid: %d\n", validCount)
	if invalidCount > 0 {
		fmt.Printf("Invalid: %d\n", invalidCount)
		return fmt.Errorf("validation failed for %d skills", invalidCount)
	}

	fmt.Println("All skills are valid")
	return nil
}

func runSkillsCreate(cmd *cobra.Command, args []string) error {
	skillID := args[0]

	workspace, err := config.LoadWorkspaceFromPath(skillsWorkspaceDir)
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w\n%s", err, errWorkspaceHint)
	}

	skillsDir := resolveSkillsDir(workspace)
	loader := skills.NewLoader(skillsDir)

	skill, err := loader.CreateSkill(skillID)
	if err != nil {
		return fmt.Errorf("failed to create skill: %w", err)
	}

	skillPath := filepath.Join(skillsDir, skill.ID, "SKILL.md")
	fmt.Printf("Skill created: %s\n", skillPath)
	fmt.Println()
	fmt.Printf("  ID:   %s\n", skill.ID)
	fmt.Printf("  Name: %s\n", skill.Name)
	fmt.Println()
	fmt.Println("Edit the file above to define behavior, tools, and examples.")
	fmt.Printf("Then enable the skill with:\n  soulgate skills configure enable %s\n", skill.ID)
	return nil
}

func runSkillsConfigureList(cmd *cobra.Command, args []string) error {
	workspace, err := config.LoadWorkspaceFromPath(skillsWorkspaceDir)
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w\n%s", err, errWorkspaceHint)
	}

	skillsDir := resolveSkillsDir(workspace)
	loader := skills.NewLoader(skillsDir)

	allIDs, err := loader.ListSkills()
	if err != nil {
		return fmt.Errorf("failed to list skills: %w", err)
	}

	if len(allIDs) == 0 {
		fmt.Println("No skills found")
		fmt.Printf("Skills directory: %s\n", skillsDir)
		fmt.Println()
		fmt.Println("Create a skill with:")
		fmt.Println("  soulgate skills create <skill-id>")
		return nil
	}

	enabledSet := enabledSkillSet(workspace.Config)
	allEnabled := len(enabledSet) == 0 // empty list means all skills are active

	fmt.Printf("Skills (%d found):\n\n", len(allIDs))
	for _, id := range allIDs {
		state := "enabled"
		if !allEnabled {
			if !enabledSet[id] {
				state = "disabled"
			}
		}
		fmt.Printf("  %-30s  %s\n", id, state)
	}

	fmt.Println()
	if allEnabled {
		fmt.Println("All discovered skills are active (no explicit enabled_skills list).")
		fmt.Println("Use 'skills configure enable' to switch to an explicit allow-list.")
	} else {
		fmt.Printf("Enabled skills list: %s\n",
			strings.Join(workspace.Config.Skills.EnabledSkills, ", "))
	}

	return nil
}

func runSkillsConfigureEnable(cmd *cobra.Command, args []string) error {
	skillID := args[0]

	workspace, err := config.LoadWorkspaceFromPath(skillsWorkspaceDir)
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w\n%s", err, errWorkspaceHint)
	}

	// Confirm the skill actually exists before touching config.
	skillsDir := resolveSkillsDir(workspace)
	loader := skills.NewLoader(skillsDir)
	if _, err := loader.Load(skillID); err != nil {
		return fmt.Errorf("skill not found: %s\n\nCreate it first with:\n  soulgate skills create %s", skillID, skillID)
	}

	// Idempotent: add only if not already present.
	cfg := workspace.Config
	if !enabledSkillSet(cfg)[skillID] {
		cfg.Skills.EnabledSkills = append(cfg.Skills.EnabledSkills, skillID)
	}

	if err := workspace.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Skill enabled: %s\n", skillID)
	return nil
}

func runSkillsConfigureDisable(cmd *cobra.Command, args []string) error {
	skillID := args[0]

	workspace, err := config.LoadWorkspaceFromPath(skillsWorkspaceDir)
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w\n%s", err, errWorkspaceHint)
	}

	cfg := workspace.Config
	set := enabledSkillSet(cfg)

	if len(set) == 0 {
		// Currently implicitly all-enabled. Switching to an explicit list that
		// excludes the requested skill means we must enumerate the existing ones.
		skillsDir := resolveSkillsDir(workspace)
		loader := skills.NewLoader(skillsDir)
		allIDs, err := loader.ListSkills()
		if err != nil {
			return fmt.Errorf("failed to list skills: %w", err)
		}
		for _, id := range allIDs {
			if id != skillID {
				cfg.Skills.EnabledSkills = append(cfg.Skills.EnabledSkills, id)
			}
		}
	} else {
		// Remove from explicit list.
		var kept []string
		for _, id := range cfg.Skills.EnabledSkills {
			if id != skillID {
				kept = append(kept, id)
			}
		}
		cfg.Skills.EnabledSkills = kept
	}

	if err := workspace.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Skill disabled: %s\n", skillID)
	return nil
}

// resolveSkillsDir returns the absolute skills directory for a workspace,
// preferring the configured path and falling back to "<root>/skills".
func resolveSkillsDir(workspace *config.Workspace) string {
	dir := workspace.Config.Skills.Dir
	if dir == "" {
		dir = "skills"
	}
	if filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(workspace.Root, dir)
}

// enabledSkillSet converts the EnabledSkills slice to a fast-lookup set.
// An empty return value means all skills are implicitly active.
func enabledSkillSet(cfg *config.Config) map[string]bool {
	set := make(map[string]bool, len(cfg.Skills.EnabledSkills))
	for _, id := range cfg.Skills.EnabledSkills {
		set[id] = true
	}
	return set
}
