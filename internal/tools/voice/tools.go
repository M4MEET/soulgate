package voice

import (
	"context"
	"encoding/json"
	"fmt"
)

// Schema mirrors the web.Schema type: a tool definition without importing the
// model package, keeping this package free of internal SoulGate dependencies.
type Schema struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ToolSchemas returns the JSON tool schema definitions for voice_speak and
// voice_transcribe. The caller converts these into model.ToolSchema values and
// appends them to the list returned by getAllToolSchemas().
func ToolSchemas() []Schema {
	return []Schema{
		{
			Name:        "voice_speak",
			Description: "Convert text to speech and save the audio as an MP3 file in the workspace. Uses OpenAI TTS (tts-1). Requires OPENAI_API_KEY.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"text": {
						"type": "string",
						"description": "The text to synthesise into speech."
					},
					"voice": {
						"type": "string",
						"description": "Voice to use. Defaults to 'nova'.",
						"enum": ["alloy", "echo", "fable", "onyx", "nova", "shimmer"]
					},
					"output": {
						"type": "string",
						"description": "Workspace-relative output path for the MP3 file, e.g. 'audio/greeting.mp3'."
					}
				},
				"required": ["text", "output"]
			}`),
		},
		{
			Name:        "voice_transcribe",
			Description: "Transcribe an audio file to text using OpenAI Whisper. Supports MP3, MP4, MPEG, MPGA, M4A, WAV, WEBM (max 25 MB). Requires OPENAI_API_KEY.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Workspace-relative path to the audio file, e.g. 'audio/recording.mp3'."
					}
				},
				"required": ["path"]
			}`),
		},
	}
}

// ExecuteTool dispatches a named voice tool call with the supplied arguments.
// workspaceRoot is the absolute path to the workspace root directory.
// apiKey is the OpenAI API key from config (may be empty; env var fallback applies).
func ExecuteTool(ctx context.Context, workspaceRoot, apiKey, name string, rawArgs json.RawMessage) (string, error) {
	switch name {
	case "voice_speak":
		return executeSpeak(ctx, workspaceRoot, apiKey, rawArgs)
	case "voice_transcribe":
		return executeTranscribe(ctx, workspaceRoot, apiKey, rawArgs)
	default:
		return "", fmt.Errorf("voice: unknown tool %q", name)
	}
}

// executeSpeak parses the arguments for voice_speak, delegates to Speak, and
// serialises the result as a JSON string.
func executeSpeak(ctx context.Context, workspaceRoot, apiKey string, rawArgs json.RawMessage) (string, error) {
	var opts SpeakOptions
	if err := json.Unmarshal(rawArgs, &opts); err != nil {
		return "", fmt.Errorf("voice_speak: invalid arguments: %w", err)
	}

	result, err := Speak(ctx, workspaceRoot, apiKey, opts)
	if err != nil {
		return "", err
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("voice_speak: failed to serialise result: %w", err)
	}
	return string(out), nil
}

// executeTranscribe parses the arguments for voice_transcribe, delegates to
// Transcribe, and serialises the result as a JSON string.
func executeTranscribe(ctx context.Context, workspaceRoot, apiKey string, rawArgs json.RawMessage) (string, error) {
	var opts TranscribeOptions
	if err := json.Unmarshal(rawArgs, &opts); err != nil {
		return "", fmt.Errorf("voice_transcribe: invalid arguments: %w", err)
	}

	result, err := Transcribe(ctx, workspaceRoot, apiKey, opts)
	if err != nil {
		return "", err
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("voice_transcribe: failed to serialise result: %w", err)
	}
	return string(out), nil
}
