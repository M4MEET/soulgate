// Package computer provides model-agnostic macOS desktop-automation helpers
// that allow the AI agent to observe and interact with the screen.
//
// Operations are performed through shell commands and AppleScript so no CGO or
// native bindings are required.  All functions are safe to call concurrently.
//
// Security note: these capabilities let the AI control the mouse and keyboard.
// They should only be enabled by an explicit policy rule (action "computer.*").
package computer

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Screenshot captures a full-screen PNG to /tmp/soulgate-screen.png and
// returns its absolute path.  It uses the macOS `screencapture` utility.
func Screenshot() (string, error) {
	out := filepath.Join(os.TempDir(), "soulgate-screen.png")
	cmd := exec.Command("screencapture", "-x", out)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("screenshot failed: %w — %s", err, strings.TrimSpace(string(output)))
	}
	return out, nil
}

// ScreenshotBase64 takes a screenshot and returns its content base64-encoded.
// Useful for embedding in model messages directly.
func ScreenshotBase64() (string, error) {
	path, err := Screenshot()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read screenshot: %w", err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// GetScreenSize returns the primary display dimensions in pixels.
// It reads them from the `system_profiler SPDisplaysDataType` output.
// On failure it returns a best-guess 1920×1080.
func GetScreenSize() (width, height int, err error) {
	out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output()
	if err != nil {
		return 1920, 1080, nil // safe fallback
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// "Resolution: 2560 x 1600 Retina"
		if strings.HasPrefix(line, "Resolution:") {
			parts := strings.Fields(line)
			// parts[0]=Resolution: parts[1]=W parts[2]=x parts[3]=H ...
			if len(parts) >= 4 {
				w, werr := strconv.Atoi(parts[1])
				h, herr := strconv.Atoi(parts[3])
				if werr == nil && herr == nil {
					return w, h, nil
				}
			}
		}
	}
	return 1920, 1080, nil
}

// Click performs a mouse click at the given screen coordinates.
// It tries `cliclick` first (a lightweight third-party tool) and falls back
// to AppleScript if cliclick is not installed.
func Click(x, y int) error {
	// Prefer cliclick — simpler and more reliable for basic clicks.
	if path, err := exec.LookPath("cliclick"); err == nil {
		coord := fmt.Sprintf("c:%d,%d", x, y)
		cmd := exec.Command(path, coord)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("cliclick click failed: %w — %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	// Fallback: AppleScript
	script := fmt.Sprintf(`
tell application "System Events"
    click at {%d, %d}
end tell`, x, y)
	return runAppleScript(script)
}

// Type sends keystrokes for text at the current cursor position.
// It uses AppleScript `keystroke` which works in the focused application.
func Type(text string) error {
	// Escape backslashes and double-quotes for the AppleScript string literal.
	escaped := strings.ReplaceAll(text, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	script := fmt.Sprintf(`
tell application "System Events"
    keystroke "%s"
end tell`, escaped)
	return runAppleScript(script)
}

// MoveMouse moves the pointer to (x, y) without clicking.
// Tries cliclick first, then AppleScript.
func MoveMouse(x, y int) error {
	if path, err := exec.LookPath("cliclick"); err == nil {
		coord := fmt.Sprintf("m:%d,%d", x, y)
		cmd := exec.Command(path, coord)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("cliclick move failed: %w — %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	script := fmt.Sprintf(`
tell application "System Events"
    set theCoords to {%d, %d}
    -- AppleScript does not expose a direct mouse-move primitive; use cliclick.
end tell`, x, y)
	_ = script
	return fmt.Errorf("mouse move requires cliclick (install with: brew install cliclick)")
}

// FindText is a placeholder for OCR-based text location.
// A real implementation would use Vision framework or an external OCR tool.
func FindText(imagePath, text string) error {
	return fmt.Errorf("FindText: OCR not yet implemented — use computer_look to ask the LLM to identify coordinates")
}

// runAppleScript executes a multi-line AppleScript via `osascript -e`.
func runAppleScript(script string) error {
	cmd := exec.Command("osascript", "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("osascript failed: %w — %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
