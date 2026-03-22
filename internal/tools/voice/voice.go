// Package voice provides text-to-speech (TTS) and speech-to-text (STT) tools
// for the SoulGate tool system. Both tools use OpenAI's audio APIs via standard
// net/http — no external SDK dependency is introduced.
//
// API key resolution order:
//  1. Explicit apiKey argument (passed by the orchestrator from config).
//  2. OPENAI_API_KEY environment variable (fallback for Anthropic-primary users).
package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	ttsEndpoint            = "https://api.openai.com/v1/audio/speech"
	transcriptionsEndpoint = "https://api.openai.com/v1/audio/transcriptions"
	ttsModel               = "tts-1"
	sttModel               = "whisper-1"
	defaultVoice           = "nova"
	voiceAPITimeout        = 60 * time.Second
	maxAudioFileSize       = 25 * 1024 * 1024 // 25 MB — OpenAI Whisper limit
)

// validVoices is the set of voices supported by OpenAI TTS.
var validVoices = map[string]bool{
	"alloy": true, "echo": true, "fable": true,
	"onyx": true, "nova": true, "shimmer": true,
}

// voiceClient is the shared HTTP client for all voice API calls.
// The 60-second timeout accommodates TTS synthesis of moderately long text.
var voiceClient = &http.Client{
	Timeout: voiceAPITimeout,
}

// SpeakOptions configures a text-to-speech request.
type SpeakOptions struct {
	// Text is the content to synthesise. Required.
	Text string `json:"text"`

	// Voice selects the OpenAI voice. Defaults to "nova".
	// Valid values: alloy, echo, fable, onyx, nova, shimmer.
	Voice string `json:"voice,omitempty"`

	// Output is the relative file path where the MP3 will be saved.
	// Required. Must not contain ".." path components.
	Output string `json:"output"`
}

// SpeakResult is returned after a successful TTS call.
type SpeakResult struct {
	// Path is the workspace-relative path of the written MP3 file.
	Path string `json:"path"`

	// Bytes is the number of audio bytes written.
	Bytes int `json:"bytes"`
}

// TranscribeOptions configures a speech-to-text request.
type TranscribeOptions struct {
	// Path is the workspace-relative path to the audio file. Required.
	Path string `json:"path"`
}

// TranscribeResult is returned after a successful STT call.
type TranscribeResult struct {
	// Text is the transcribed content.
	Text string `json:"text"`
}

// Speak converts text to speech and saves the resulting MP3 to workspaceRoot/opts.Output.
// apiKey may be empty; in that case the OPENAI_API_KEY environment variable is used.
func Speak(ctx context.Context, workspaceRoot, apiKey string, opts SpeakOptions) (*SpeakResult, error) {
	if opts.Text == "" {
		return nil, fmt.Errorf("voice_speak: text is required")
	}
	if opts.Output == "" {
		return nil, fmt.Errorf("voice_speak: output path is required")
	}
	if strings.Contains(opts.Output, "..") {
		return nil, fmt.Errorf("voice_speak: path traversal detected in output %q", opts.Output)
	}

	voice := opts.Voice
	if voice == "" {
		voice = defaultVoice
	}
	if !validVoices[voice] {
		return nil, fmt.Errorf("voice_speak: unsupported voice %q (valid: alloy, echo, fable, onyx, nova, shimmer)", voice)
	}

	key := resolveAPIKey(apiKey)
	if key == "" {
		return nil, fmt.Errorf("voice_speak: no OpenAI API key available; set OPENAI_API_KEY or configure openai.api_key")
	}

	// Build request body.
	reqBody, err := json.Marshal(map[string]string{
		"model": ttsModel,
		"input": opts.Text,
		"voice": voice,
	})
	if err != nil {
		return nil, fmt.Errorf("voice_speak: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ttsEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("voice_speak: failed to build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := voiceClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("voice_speak: API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("voice_speak: OpenAI TTS returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Read audio bytes.
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("voice_speak: failed to read audio response: %w", err)
	}

	// Write to workspace.
	destPath := filepath.Join(workspaceRoot, filepath.Clean(opts.Output))
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return nil, fmt.Errorf("voice_speak: failed to create output directory: %w", err)
	}
	if err := os.WriteFile(destPath, audioData, 0644); err != nil {
		return nil, fmt.Errorf("voice_speak: failed to write audio file: %w", err)
	}

	return &SpeakResult{
		Path:  opts.Output,
		Bytes: len(audioData),
	}, nil
}

// Transcribe sends an audio file to OpenAI Whisper and returns the transcribed text.
// apiKey may be empty; in that case the OPENAI_API_KEY environment variable is used.
func Transcribe(ctx context.Context, workspaceRoot, apiKey string, opts TranscribeOptions) (*TranscribeResult, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("voice_transcribe: path is required")
	}
	if strings.Contains(opts.Path, "..") {
		return nil, fmt.Errorf("voice_transcribe: path traversal detected in %q", opts.Path)
	}

	key := resolveAPIKey(apiKey)
	if key == "" {
		return nil, fmt.Errorf("voice_transcribe: no OpenAI API key available; set OPENAI_API_KEY or configure openai.api_key")
	}

	// Resolve and validate the audio file path.
	audioPath := filepath.Join(workspaceRoot, filepath.Clean(opts.Path))
	info, err := os.Stat(audioPath)
	if err != nil {
		return nil, fmt.Errorf("voice_transcribe: cannot access audio file %q: %w", opts.Path, err)
	}
	if info.Size() > maxAudioFileSize {
		return nil, fmt.Errorf("voice_transcribe: file %q exceeds the 25 MB Whisper limit", opts.Path)
	}

	f, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf("voice_transcribe: cannot open audio file %q: %w", opts.Path, err)
	}
	defer f.Close()

	// Build multipart/form-data body.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// model field
	if err := mw.WriteField("model", sttModel); err != nil {
		return nil, fmt.Errorf("voice_transcribe: failed to write model field: %w", err)
	}

	// file field — use the basename as the filename so Whisper can infer format.
	fw, err := mw.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return nil, fmt.Errorf("voice_transcribe: failed to create form file: %w", err)
	}
	if _, err := io.Copy(fw, f); err != nil {
		return nil, fmt.Errorf("voice_transcribe: failed to copy audio data: %w", err)
	}

	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("voice_transcribe: failed to close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, transcriptionsEndpoint, &buf)
	if err != nil {
		return nil, fmt.Errorf("voice_transcribe: failed to build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := voiceClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("voice_transcribe: API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("voice_transcribe: OpenAI Whisper returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("voice_transcribe: failed to decode response: %w", err)
	}

	return &TranscribeResult{Text: apiResp.Text}, nil
}

// resolveAPIKey returns the provided key if non-empty, otherwise falls back to
// the OPENAI_API_KEY environment variable. This ensures Anthropic-primary users
// can still use voice tools by setting only the environment variable.
func resolveAPIKey(configured string) string {
	if configured != "" {
		return configured
	}
	return os.Getenv("OPENAI_API_KEY")
}
