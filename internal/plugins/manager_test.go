package plugins

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeManifest(t *testing.T, dir, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.yml"), []byte(content), 0644))
}

func writeScript(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0755))
}

func TestManagerLoadAll(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid script plugin
	pluginDir := filepath.Join(tmpDir, "hello")
	writeManifest(t, pluginDir, `
name: hello
version: 1.0.0
description: Test plugin
runtime: script
tools:
  - name: greet
    description: Says hello
    command: echo '{"greeting":"hello"}'
    input_schema:
      type: object
      properties:
        name:
          type: string
`)

	mgr := NewManager(tmpDir, 10, 0)
	require.NoError(t, mgr.LoadAll())

	// Should have loaded 1 plugin
	assert.Equal(t, 1, len(mgr.ListPlugins()))
	assert.Contains(t, mgr.ListPlugins(), "hello")

	// Should have 1 tool schema
	schemas := mgr.GetToolSchemas()
	assert.Equal(t, 1, len(schemas))
	assert.Equal(t, "hello__greet", schemas[0].Name)
	assert.Contains(t, schemas[0].Description, "Says hello")
}

func TestManagerIsPluginTool(t *testing.T) {
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "calc")
	writeManifest(t, pluginDir, `
name: calc
version: 1.0.0
tools:
  - name: add
    description: Adds numbers
    command: echo 42
    input_schema:
      type: object
      properties:
        a:
          type: number
`)
	mgr := NewManager(tmpDir, 10, 0)
	require.NoError(t, mgr.LoadAll())

	assert.True(t, mgr.IsPluginTool("calc__add"))
	assert.False(t, mgr.IsPluginTool("calc__subtract"))
	assert.False(t, mgr.IsPluginTool("files_read"))
}

func TestManagerExecuteScriptTool(t *testing.T) {
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "echo")

	writeManifest(t, pluginDir, `
name: echo
version: 1.0.0
runtime: script
tools:
  - name: say
    description: Echoes message
    command: python3 echo.py
    input_schema:
      type: object
      properties:
        message:
          type: string
      required: [message]
requires:
  bins: [python3]
`)

	writeScript(t, pluginDir, "echo.py", `
import json, sys
data = json.load(sys.stdin)
print(json.dumps({"echoed": data.get("message", "")}))
`)

	mgr := NewManager(tmpDir, 10, 0)
	require.NoError(t, mgr.LoadAll())

	input, _ := json.Marshal(map[string]string{"message": "hello world"})
	result, err := mgr.ExecuteTool(context.Background(), "echo__say", input)
	require.NoError(t, err)

	var output map[string]string
	require.NoError(t, json.Unmarshal([]byte(result), &output))
	assert.Equal(t, "hello world", output["echoed"])
}

func TestManagerReload(t *testing.T) {
	tmpDir := t.TempDir()

	mgr := NewManager(tmpDir, 10, 0)
	require.NoError(t, mgr.LoadAll())
	assert.Equal(t, 0, len(mgr.ListPlugins()))

	// Create a plugin after initial load
	pluginDir := filepath.Join(tmpDir, "newplugin")
	writeManifest(t, pluginDir, `
name: newplugin
version: 1.0.0
tools:
  - name: do_thing
    description: Does a thing
    command: echo done
    input_schema:
      type: object
`)

	// Reload should pick it up
	require.NoError(t, mgr.Reload())
	assert.Equal(t, 1, len(mgr.ListPlugins()))
	assert.True(t, mgr.IsPluginTool("newplugin__do_thing"))
}

func TestManagerInvalidManifest(t *testing.T) {
	tmpDir := t.TempDir()

	// Missing name
	pluginDir := filepath.Join(tmpDir, "bad")
	writeManifest(t, pluginDir, `
version: 1.0.0
tools:
  - name: x
    description: y
    input_schema:
      type: object
`)

	mgr := NewManager(tmpDir, 10, 0)
	require.NoError(t, mgr.LoadAll()) // doesn't error, just warns
	assert.Equal(t, 0, len(mgr.ListPlugins()))
}

func TestManagerEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "nope")

	mgr := NewManager(nonExistent, 10, 0)
	require.NoError(t, mgr.LoadAll())
	assert.Equal(t, 0, len(mgr.ListPlugins()))
}

func TestManagerScriptTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "slow")

	writeManifest(t, pluginDir, `
name: slow
version: 1.0.0
runtime: script
tools:
  - name: hang
    description: Takes forever
    command: sleep 60
    input_schema:
      type: object
`)

	mgr := NewManager(tmpDir, 1, 0) // 1 second timeout
	require.NoError(t, mgr.LoadAll())

	_, err := mgr.ExecuteTool(context.Background(), "slow__hang", []byte("{}"))
	assert.Error(t, err)
}

func TestManagerScriptStderr(t *testing.T) {
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "failing")

	writeManifest(t, pluginDir, `
name: failing
version: 1.0.0
runtime: script
tools:
  - name: fail
    description: Fails with error
    command: python3 fail.py
    input_schema:
      type: object
requires:
  bins: [python3]
`)

	writeScript(t, pluginDir, "fail.py", `
import sys
print("something went wrong", file=sys.stderr)
sys.exit(1)
`)

	mgr := NewManager(tmpDir, 10, 0)
	require.NoError(t, mgr.LoadAll())

	_, err := mgr.ExecuteTool(context.Background(), "failing__fail", []byte("{}"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "something went wrong")
}
