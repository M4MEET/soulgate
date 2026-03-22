package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/model"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose SoulGate configuration and connectivity",
	Long: `Run diagnostic checks on your SoulGate installation.

Checks:
  - Configuration files exist and are valid
  - API keys are configured
  - Model providers are reachable
  - Dependencies are installed
  - Gateway connectivity (if running)

Example:
  soulgate doctor`,
	Run: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

type checkResult struct {
	name   string
	status string // "ok", "warn", "fail"
	detail string
}

func runDoctor(cmd *cobra.Command, args []string) {
	fmt.Println()
	fmt.Println("  🩺 SoulGate Doctor")
	fmt.Println("  ══════════════════")
	fmt.Println()

	var results []checkResult

	// 1. System info
	fmt.Printf("  %-30s %s/%s\n", "Platform:", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  %-30s %s\n", "Go version:", runtime.Version())
	fmt.Println()

	// 2. Config directory
	homeDir, _ := os.UserHomeDir()
	globalConfigDir := filepath.Join(homeDir, ".soulgate")
	localConfigDir := ".soulgate"

	results = append(results, checkDir("Global config (~/.soulgate)", globalConfigDir))
	results = append(results, checkDir("Local config (.soulgate)", localConfigDir))

	// 3. Config file
	configPaths := []string{
		filepath.Join(localConfigDir, "config.yml"),
		filepath.Join(globalConfigDir, "config.yml"),
	}
	var loadedConfig *config.Config
	for _, p := range configPaths {
		if _, err := os.Stat(p); err == nil {
			cfg, err := config.LoadConfig(p)
			if err != nil {
				results = append(results, checkResult{"Config (" + p + ")", "fail", err.Error()})
			} else {
				results = append(results, checkResult{"Config (" + p + ")", "ok", "valid YAML"})
				loadedConfig = cfg
			}
			break
		}
	}
	if loadedConfig == nil {
		results = append(results, checkResult{"Config file", "warn", "no config.yml found — run 'soulgate tui' to set up"})
	}

	// 4. Policy file
	if loadedConfig != nil {
		policyPath := loadedConfig.Policy.FilePath
		if _, err := os.Stat(policyPath); err == nil {
			results = append(results, checkResult{"Policy file", "ok", policyPath})
		} else {
			results = append(results, checkResult{"Policy file", "warn", "not found — using default-deny"})
		}
	}

	// 5. File permission checks — sensitive .soulgate files must be owner-only
	for _, permCheck := range []struct{ label, path string }{
		{"Global config dir (~/.soulgate)", globalConfigDir},
		{"~/.soulgate/config.yml", filepath.Join(globalConfigDir, "config.yml")},
		{".soulgate/config.yml", filepath.Join(localConfigDir, "config.yml")},
	} {
		info, err := os.Stat(permCheck.path)
		if err != nil {
			continue // path doesn't exist; skip
		}
		perm := info.Mode().Perm()
		if perm&0077 != 0 {
			results = append(results, checkResult{
				"Permissions: " + permCheck.label,
				"warn",
				fmt.Sprintf("world/group readable (%04o) — run: chmod %s %s",
					perm,
					map[bool]string{true: "700", false: "600"}[info.IsDir()],
					permCheck.path),
			})
		} else {
			results = append(results, checkResult{"Permissions: " + permCheck.label, "ok",
				fmt.Sprintf("%04o", perm)})
		}
	}

	// 6. API keys
	if loadedConfig != nil {
		provider := loadedConfig.Model.DefaultProvider
		results = append(results, checkResult{"Default provider", "ok", provider})

		// Check provider-specific key
		switch provider {
		case "anthropic":
			key := loadedConfig.Model.Anthropic.APIKey
			if key == "" {
				key = os.Getenv("ANTHROPIC_API_KEY")
			}
			if key != "" {
				results = append(results, checkResult{"Anthropic API key", "ok", key[:8] + "..." + key[len(key)-4:]})
			} else {
				results = append(results, checkResult{"Anthropic API key", "fail", "not set — set ANTHROPIC_API_KEY or configure in config.yml"})
			}
		case "openai":
			key := loadedConfig.Model.OpenAI.APIKey
			if key == "" {
				key = os.Getenv("OPENAI_API_KEY")
			}
			if key != "" {
				results = append(results, checkResult{"OpenAI API key", "ok", key[:8] + "..." + key[len(key)-4:]})
			} else {
				results = append(results, checkResult{"OpenAI API key", "fail", "not set — set OPENAI_API_KEY or configure in config.yml"})
			}
		case "ollama":
			results = append(results, checkResult{"Ollama", "ok", "no API key needed"})
		default:
			// Check env for known providers
			def, err := model.LookupProvider(provider)
			if err == nil && def.EnvKey != "" {
				key := os.Getenv(def.EnvKey)
				if key != "" {
					results = append(results, checkResult{provider + " API key", "ok", "set via " + def.EnvKey})
				} else {
					results = append(results, checkResult{provider + " API key", "warn", "not set — set " + def.EnvKey})
				}
			}
		}
	}

	// 7. Check all registered providers
	fmt.Println()
	fmt.Println("  Registered Providers:")
	for _, name := range model.AllProviderNames() {
		def, _ := model.LookupProvider(name)
		keyStatus := "no key"
		if def.EnvKey == "" {
			keyStatus = "no key needed"
		} else if os.Getenv(def.EnvKey) != "" {
			keyStatus = "✓ key set"
		} else if loadedConfig != nil {
			switch name {
			case "openai":
				if loadedConfig.Model.OpenAI.APIKey != "" {
					keyStatus = "✓ in config"
				}
			case "anthropic":
				if loadedConfig.Model.Anthropic.APIKey != "" {
					keyStatus = "✓ in config"
				}
			}
		}
		fmt.Printf("    %-15s %-30s %s\n", name, def.DefaultModel, keyStatus)
	}

	// 8. Gateway connectivity
	fmt.Println()
	gwURL := "http://localhost:8080"
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(gwURL + "/api/health")
	if err != nil {
		results = append(results, checkResult{"Gateway (localhost:8080)", "warn", "not running"})
	} else {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			results = append(results, checkResult{"Gateway (localhost:8080)", "ok", "healthy"})
		} else {
			results = append(results, checkResult{"Gateway (localhost:8080)", "warn", fmt.Sprintf("status %d", resp.StatusCode)})
		}
	}

	// 9. External tools
	for _, tool := range []struct{ name, check string }{
		{"git", "git --version"},
		{"go", "go version"},
		{"ffmpeg", "ffmpeg -version"},
		{"node", "node --version"},
	} {
		parts := strings.Fields(tool.check)
		out, err := exec.Command(parts[0], parts[1:]...).Output()
		if err != nil {
			results = append(results, checkResult{tool.name, "warn", "not installed"})
		} else {
			ver := strings.TrimSpace(strings.Split(string(out), "\n")[0])
			if len(ver) > 50 {
				ver = ver[:50] + "..."
			}
			results = append(results, checkResult{tool.name, "ok", ver})
		}
	}

	// Print results
	fmt.Println()
	fmt.Println("  Checks:")
	okCount, warnCount, failCount := 0, 0, 0
	for _, r := range results {
		var icon string
		switch r.status {
		case "ok":
			icon = "\033[32m✓\033[0m"
			okCount++
		case "warn":
			icon = "\033[33m!\033[0m"
			warnCount++
		case "fail":
			icon = "\033[31m✗\033[0m"
			failCount++
		}
		fmt.Printf("    %s %-30s %s\n", icon, r.name, r.detail)
	}

	fmt.Println()
	fmt.Printf("  Summary: %d passed", okCount)
	if warnCount > 0 {
		fmt.Printf(", %d warnings", warnCount)
	}
	if failCount > 0 {
		fmt.Printf(", %d failed", failCount)
	}
	fmt.Println()
	fmt.Println()
}

func checkDir(name, path string) checkResult {
	if _, err := os.Stat(path); err == nil {
		return checkResult{name, "ok", path}
	}
	return checkResult{name, "warn", "not found"}
}
