package loader

import (
	"fmt"

	"github.com/M4MEET/soulgate/internal/plugins/sdk"
)

// ValidateManifest validates a plugin manifest
func ValidateManifest(manifest *sdk.Manifest) error {
	if manifest.Name == "" {
		return fmt.Errorf("plugin name is required")
	}

	if manifest.Version == "" {
		return fmt.Errorf("plugin version is required")
	}

	// Default runtime is "script"; accept script, wasm, or empty
	switch manifest.Runtime {
	case "", "script", "wasm":
		// ok
	default:
		return fmt.Errorf("unsupported runtime: %s (use 'script' or 'wasm')", manifest.Runtime)
	}

	if len(manifest.Tools) == 0 {
		return fmt.Errorf("plugin must provide at least one tool")
	}

	// Validate tools
	toolNames := make(map[string]bool)
	for i, tool := range manifest.Tools {
		if tool.Name == "" {
			return fmt.Errorf("tool %d: name is required", i)
		}

		// Check for duplicate tool names
		if toolNames[tool.Name] {
			return fmt.Errorf("duplicate tool name: %s", tool.Name)
		}
		toolNames[tool.Name] = true

		if tool.Description == "" {
			return fmt.Errorf("tool %s: description is required", tool.Name)
		}

		if len(tool.InputSchema) == 0 {
			return fmt.Errorf("tool %s: input_schema is required", tool.Name)
		}
	}

	return nil
}
