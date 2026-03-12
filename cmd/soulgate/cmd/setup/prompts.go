package setup

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ============================================================================
// Interactive Prompt Functions
// ============================================================================

func promptString(reader *bufio.Reader, prompt, defaultValue, hint string) string {
	for {
		if hint != "" {
			fmt.Printf("\n  %s\n", colorGray(hint))
		}

		if defaultValue != "" {
			fmt.Printf("%s [%s]: ", prompt, colorCyan(defaultValue))
		} else {
			fmt.Printf("%s: ", prompt)
		}

		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("\n%s Error reading input: %v\n", colorRed("✗"), err)
			continue
		}

		input = strings.TrimSpace(input)

		if input == "" {
			if defaultValue != "" {
				return defaultValue
			}
			fmt.Printf("%s Please enter a value\n", colorYellow("⚠"))
			continue
		}

		return input
	}
}

func promptYesNo(reader *bufio.Reader, prompt string, defaultValue bool, hint string) bool {
	defaultStr := "y/N"
	if defaultValue {
		defaultStr = "Y/n"
	}

	if hint != "" {
		fmt.Printf("\n  %s\n", colorGray(hint))
	}
	fmt.Printf("%s [%s]: ", prompt, colorCyan(defaultStr))

	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("%s Error reading input: %v\n", colorRed("✗"), err)
			fmt.Printf("%s [%s]: ", prompt, colorCyan(defaultStr))
			continue
		}

		input = strings.TrimSpace(strings.ToLower(input))

		if input == "" {
			return defaultValue
		}

		if input == "y" || input == "yes" {
			return true
		}
		if input == "n" || input == "no" {
			return false
		}

		fmt.Printf("%s Please enter 'y' or 'n'\n", colorYellow("⚠"))
		fmt.Printf("%s [%s]: ", prompt, colorCyan(defaultStr))
	}
}

func promptInt(reader *bufio.Reader, prompt string, defaultValue, min, max int, hint string) int {
	for {
		if hint != "" {
			fmt.Printf("\n  %s\n", colorGray(hint))
		}
		fmt.Printf("%s [%s]: ", prompt, colorCyan(fmt.Sprintf("%d", defaultValue)))

		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("%s Error reading input: %v\n", colorRed("✗"), err)
			continue
		}

		input = strings.TrimSpace(input)

		if input == "" {
			return defaultValue
		}

		value, err := strconv.Atoi(input)
		if err != nil {
			fmt.Printf("%s Invalid number, please try again\n", colorYellow("⚠"))
			continue
		}

		if value < min || value > max {
			fmt.Printf("%s Please enter a number between %d and %d\n", colorYellow("⚠"), min, max)
			continue
		}

		return value
	}
}

func promptChoice(reader *bufio.Reader, prompt string, choices []string, defaultValue string) string {
	fmt.Printf("\n%s\n", prompt)

	defaultIndex := 0
	for i, choice := range choices {
		marker := "  "
		if choice == defaultValue {
			marker = colorCyan("▸")
			defaultIndex = i + 1
		} else {
			marker = " "
		}
		fmt.Printf("  %s %s) %s\n", marker, colorCyan(fmt.Sprintf("%d", i+1)), choice)
	}

	fmt.Printf("\nChoice [%s]: ", colorCyan(fmt.Sprintf("%d", defaultIndex)))

	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("%s Error reading input: %v\n", colorRed("✗"), err)
			fmt.Printf("Choice [%s]: ", colorCyan(fmt.Sprintf("%d", defaultIndex)))
			continue
		}

		input = strings.TrimSpace(input)

		if input == "" {
			return defaultValue
		}

		index, err := strconv.Atoi(input)
		if err != nil || index < 1 || index > len(choices) {
			fmt.Printf("%s Please enter a number between 1 and %d\n", colorYellow("⚠"), len(choices))
			fmt.Printf("Choice [%s]: ", colorCyan(fmt.Sprintf("%d", defaultIndex)))
			continue
		}

		return choices[index-1]
	}
}

// ============================================================================
// Formatting Helpers
// ============================================================================

func formatBool(value bool) string {
	if value {
		return colorGreen("✓ Enabled")
	}
	return colorRed("✗ Disabled")
}

func formatCheckmark(success bool) string {
	if success {
		return colorGreen("✓")
	}
	return colorRed("✗")
}

func formatPaths(paths []string) string {
	if len(paths) == 0 {
		return colorGray("(none)")
	}
	if len(paths) == 1 {
		return paths[0]
	}

	// Show first path, then count
	return fmt.Sprintf("%s (+ %d more)", paths[0], len(paths)-1)
}

func formatChannels(channels []string) string {
	if len(channels) == 0 {
		return colorGray("(none)")
	}
	return strings.Join(channels, ", ")
}

// ============================================================================
// Validation Helpers
// ============================================================================

func validateWorkspacePath(path string) error {
	// Check if parent directory exists
	parentDir := filepath.Dir(path)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		return fmt.Errorf("parent directory does not exist: %s", parentDir)
	}

	// If path exists, check if it's a directory
	if info, err := os.Stat(path); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("path exists but is not a directory")
		}
	}

	return nil
}

func validateAPIKey(key, expectedPrefix string) bool {
	if key == "" {
		return true // Empty is valid (might use env var)
	}
	return strings.HasPrefix(key, expectedPrefix)
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	path = os.ExpandEnv(path)
	if absPath, err := filepath.Abs(path); err == nil {
		return absPath
	}
	return path
}
