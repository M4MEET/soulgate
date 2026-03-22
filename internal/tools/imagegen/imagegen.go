// Package imagegen provides image generation and editing tools for SoulGate.
// Two backends are supported:
//
//   - OpenAI DALL-E 3 (POST /v1/images/generations, POST /v1/images/edits)
//   - FAL.ai (Flux / Stable Diffusion) via a generic HTTP JSON interface
//
// API key resolution order for each backend:
//  1. Explicit apiKey argument (passed by the orchestrator from config).
//  2. OPENAI_API_KEY / FAL_KEY environment variable (fallback).
package imagegen

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
	openAIGenerateEndpoint = "https://api.openai.com/v1/images/generations"
	openAIEditEndpoint     = "https://api.openai.com/v1/images/edits"
	dalleModel             = "dall-e-3"
	defaultSize            = "1024x1024"
	imageAPITimeout        = 120 * time.Second
)

// validSizes is the set of sizes supported by DALL-E 3.
var validSizes = map[string]bool{
	"1024x1024": true,
	"1024x1792": true,
	"1792x1024": true,
}

// imageClient is the shared HTTP client for all image API calls.
// The 2-minute timeout accommodates image generation latency.
var imageClient = &http.Client{
	Timeout: imageAPITimeout,
}

// GenerateOptions configures an image generation request.
type GenerateOptions struct {
	// Prompt is the text description of the image to generate. Required.
	Prompt string `json:"prompt"`

	// Size controls the output dimensions. Defaults to "1024x1024".
	// Valid values: "1024x1024", "1024x1792", "1792x1024".
	Size string `json:"size,omitempty"`

	// Output is the workspace-relative file path where the PNG will be saved.
	// Defaults to "generated-<timestamp>.png" in the workspace root.
	// Must not contain ".." path components.
	Output string `json:"output,omitempty"`

	// Provider selects the backend. "openai" (default) uses DALL-E 3.
	// "fal" uses the FAL.ai HTTP API.
	Provider string `json:"provider,omitempty"`

	// FALModel is the FAL.ai model endpoint path, e.g.
	// "fal-ai/flux/schnell". Only used when Provider is "fal".
	FALModel string `json:"fal_model,omitempty"`
}

// GenerateResult is returned after a successful image generation.
type GenerateResult struct {
	// Path is the workspace-relative path of the written PNG file.
	Path string `json:"path"`

	// RevisedPrompt is the prompt that was actually used by the model
	// (DALL-E 3 may revise the user prompt). Empty for FAL.ai.
	RevisedPrompt string `json:"revised_prompt,omitempty"`

	// Bytes is the number of image bytes written.
	Bytes int `json:"bytes"`

	// Provider is the backend that generated the image.
	Provider string `json:"provider"`
}

// EditOptions configures an image editing request (OpenAI only).
type EditOptions struct {
	// Path is the workspace-relative path to the source PNG. Required.
	Path string `json:"path"`

	// Prompt describes the edit to apply. Required.
	Prompt string `json:"prompt"`

	// Output is the workspace-relative file path for the result.
	// Defaults to "<basename>-edited-<timestamp>.png".
	// Must not contain ".." path components.
	Output string `json:"output,omitempty"`

	// Size controls the output dimensions. Defaults to "1024x1024".
	Size string `json:"size,omitempty"`
}

// EditResult is returned after a successful image edit.
type EditResult struct {
	// Path is the workspace-relative path of the written PNG file.
	Path string `json:"path"`

	// Bytes is the number of image bytes written.
	Bytes int `json:"bytes"`
}

// Generate calls the configured image backend and writes the result to disk.
// apiKey may be empty; the appropriate environment variable is used as fallback.
func Generate(ctx context.Context, workspaceRoot, apiKey string, opts GenerateOptions) (*GenerateResult, error) {
	if strings.TrimSpace(opts.Prompt) == "" {
		return nil, fmt.Errorf("image_generate: prompt is required")
	}
	if strings.Contains(opts.Output, "..") {
		return nil, fmt.Errorf("image_generate: path traversal detected in output %q", opts.Output)
	}

	size := opts.Size
	if size == "" {
		size = defaultSize
	}
	if !validSizes[size] {
		return nil, fmt.Errorf("image_generate: unsupported size %q (valid: 1024x1024, 1024x1792, 1792x1024)", size)
	}

	output := opts.Output
	if output == "" {
		output = fmt.Sprintf("generated-%d.png", time.Now().UnixMilli())
	}

	provider := strings.ToLower(strings.TrimSpace(opts.Provider))
	if provider == "" {
		provider = "openai"
	}

	switch provider {
	case "openai":
		return generateOpenAI(ctx, workspaceRoot, apiKey, opts.Prompt, size, output)
	case "fal":
		return generateFAL(ctx, workspaceRoot, apiKey, opts.Prompt, size, output, opts.FALModel)
	default:
		return nil, fmt.Errorf("image_generate: unknown provider %q (valid: openai, fal)", provider)
	}
}

// Edit sends an image to OpenAI's edit endpoint and saves the result to disk.
// apiKey may be empty; the OPENAI_API_KEY environment variable is used as fallback.
func Edit(ctx context.Context, workspaceRoot, apiKey string, opts EditOptions) (*EditResult, error) {
	if strings.TrimSpace(opts.Path) == "" {
		return nil, fmt.Errorf("image_edit: path is required")
	}
	if strings.Contains(opts.Path, "..") {
		return nil, fmt.Errorf("image_edit: path traversal detected in path %q", opts.Path)
	}
	if strings.TrimSpace(opts.Prompt) == "" {
		return nil, fmt.Errorf("image_edit: prompt is required")
	}
	if strings.Contains(opts.Output, "..") {
		return nil, fmt.Errorf("image_edit: path traversal detected in output %q", opts.Output)
	}

	size := opts.Size
	if size == "" {
		size = defaultSize
	}
	if !validSizes[size] {
		return nil, fmt.Errorf("image_edit: unsupported size %q (valid: 1024x1024, 1024x1792, 1792x1024)", size)
	}

	key := resolveOpenAIKey(apiKey)
	if key == "" {
		return nil, fmt.Errorf("image_edit: no OpenAI API key available; set OPENAI_API_KEY or configure openai.api_key")
	}

	// Resolve source image path.
	srcPath := filepath.Join(workspaceRoot, filepath.Clean(opts.Path))
	f, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("image_edit: cannot open source image %q: %w", opts.Path, err)
	}
	defer f.Close()

	// Determine output path.
	output := opts.Output
	if output == "" {
		base := strings.TrimSuffix(filepath.Base(opts.Path), filepath.Ext(opts.Path))
		output = fmt.Sprintf("%s-edited-%d.png", base, time.Now().UnixMilli())
	}

	// Build multipart form body.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	if err := mw.WriteField("model", dalleModel); err != nil {
		return nil, fmt.Errorf("image_edit: failed to write model field: %w", err)
	}
	if err := mw.WriteField("prompt", opts.Prompt); err != nil {
		return nil, fmt.Errorf("image_edit: failed to write prompt field: %w", err)
	}
	if err := mw.WriteField("n", "1"); err != nil {
		return nil, fmt.Errorf("image_edit: failed to write n field: %w", err)
	}
	if err := mw.WriteField("size", size); err != nil {
		return nil, fmt.Errorf("image_edit: failed to write size field: %w", err)
	}
	if err := mw.WriteField("response_format", "url"); err != nil {
		return nil, fmt.Errorf("image_edit: failed to write response_format field: %w", err)
	}

	fw, err := mw.CreateFormFile("image", filepath.Base(srcPath))
	if err != nil {
		return nil, fmt.Errorf("image_edit: failed to create form file: %w", err)
	}
	if _, err := io.Copy(fw, f); err != nil {
		return nil, fmt.Errorf("image_edit: failed to copy image data: %w", err)
	}

	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("image_edit: failed to close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIEditEndpoint, &buf)
	if err != nil {
		return nil, fmt.Errorf("image_edit: failed to build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := imageClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("image_edit: API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("image_edit: OpenAI returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	imageURL, _, err := decodeOpenAIImageResponse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("image_edit: %w", err)
	}

	n, err := downloadImageToWorkspace(ctx, imageURL, workspaceRoot, output)
	if err != nil {
		return nil, fmt.Errorf("image_edit: %w", err)
	}

	return &EditResult{Path: output, Bytes: n}, nil
}

// generateOpenAI calls the DALL-E 3 generations endpoint.
func generateOpenAI(ctx context.Context, workspaceRoot, apiKey, prompt, size, output string) (*GenerateResult, error) {
	key := resolveOpenAIKey(apiKey)
	if key == "" {
		return nil, fmt.Errorf("image_generate: no OpenAI API key available; set OPENAI_API_KEY or configure openai.api_key")
	}

	reqBody, err := json.Marshal(map[string]interface{}{
		"model":           dalleModel,
		"prompt":          prompt,
		"n":               1,
		"size":            size,
		"response_format": "url",
	})
	if err != nil {
		return nil, fmt.Errorf("image_generate: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIGenerateEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("image_generate: failed to build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := imageClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("image_generate: API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("image_generate: OpenAI returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	imageURL, revisedPrompt, err := decodeOpenAIImageResponse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("image_generate: %w", err)
	}

	n, err := downloadImageToWorkspace(ctx, imageURL, workspaceRoot, output)
	if err != nil {
		return nil, fmt.Errorf("image_generate: %w", err)
	}

	return &GenerateResult{
		Path:          output,
		RevisedPrompt: revisedPrompt,
		Bytes:         n,
		Provider:      "openai",
	}, nil
}

// openAIImageResponse is the envelope returned by /v1/images/generations
// and /v1/images/edits when response_format is "url".
type openAIImageResponse struct {
	Data []struct {
		URL           string `json:"url"`
		RevisedPrompt string `json:"revised_prompt"`
	} `json:"data"`
}

// decodeOpenAIImageResponse parses the OpenAI image API JSON body and returns
// the image URL and (optional) revised prompt from the first data element.
func decodeOpenAIImageResponse(r io.Reader) (imageURL, revisedPrompt string, err error) {
	var apiResp openAIImageResponse
	if err = json.NewDecoder(r).Decode(&apiResp); err != nil {
		return "", "", fmt.Errorf("failed to decode response: %w", err)
	}
	if len(apiResp.Data) == 0 || apiResp.Data[0].URL == "" {
		return "", "", fmt.Errorf("no image URL in response")
	}
	return apiResp.Data[0].URL, apiResp.Data[0].RevisedPrompt, nil
}

// generateFAL calls the FAL.ai queue API for the specified model endpoint.
// FAL uses a synchronous queue: POST to enqueue, then poll until done.
// The result URL is downloaded to the workspace.
func generateFAL(ctx context.Context, workspaceRoot, apiKey, prompt, size, output, falModel string) (*GenerateResult, error) {
	key := resolveFALKey(apiKey)
	if key == "" {
		return nil, fmt.Errorf("image_generate: no FAL API key available; set FAL_KEY or configure fal.api_key")
	}

	if falModel == "" {
		falModel = "fal-ai/flux/schnell"
	}

	// FAL.ai synchronous API: POST to /fal/queue/submit/<model>
	endpoint := "https://queue.fal.run/" + falModel

	// Parse width/height from size string (e.g., "1024x1024").
	width, height := parseSizeDimensions(size)

	reqBody, err := json.Marshal(map[string]interface{}{
		"prompt":           prompt,
		"image_size":       map[string]int{"width": width, "height": height},
		"num_images":       1,
		"enable_safety_checker": false,
	})
	if err != nil {
		return nil, fmt.Errorf("image_generate: failed to marshal FAL request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("image_generate: failed to build FAL request: %w", err)
	}
	req.Header.Set("Authorization", "Key "+key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := imageClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("image_generate: FAL request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("image_generate: FAL returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	// FAL synchronous response contains images[].url.
	var falResp struct {
		Images []struct {
			URL string `json:"url"`
		} `json:"images"`
		// Some models use "image" instead.
		Image struct {
			URL string `json:"url"`
		} `json:"image"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&falResp); err != nil {
		return nil, fmt.Errorf("image_generate: failed to decode FAL response: %w", err)
	}

	imageURL := ""
	if len(falResp.Images) > 0 && falResp.Images[0].URL != "" {
		imageURL = falResp.Images[0].URL
	} else if falResp.Image.URL != "" {
		imageURL = falResp.Image.URL
	}
	if imageURL == "" {
		return nil, fmt.Errorf("image_generate: no image URL in FAL response")
	}

	n, err := downloadImageToWorkspace(ctx, imageURL, workspaceRoot, output)
	if err != nil {
		return nil, fmt.Errorf("image_generate: %w", err)
	}

	return &GenerateResult{
		Path:     output,
		Bytes:    n,
		Provider: "fal",
	}, nil
}

// downloadImageToWorkspace fetches imageURL and writes the body to
// workspaceRoot/relPath. Returns the number of bytes written.
func downloadImageToWorkspace(ctx context.Context, imageURL, workspaceRoot, relPath string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to build download request: %w", err)
	}

	resp, err := imageClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("image download returned HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read image data: %w", err)
	}

	destPath := filepath.Join(workspaceRoot, filepath.Clean(relPath))
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return 0, fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return 0, fmt.Errorf("failed to write image file: %w", err)
	}

	return len(data), nil
}

// parseSizeDimensions splits "WIDTHxHEIGHT" into two ints.
// Falls back to 1024x1024 on malformed input.
func parseSizeDimensions(size string) (width, height int) {
	width, height = 1024, 1024
	parts := strings.SplitN(size, "x", 2)
	if len(parts) != 2 {
		return
	}
	fmt.Sscanf(parts[0], "%d", &width)
	fmt.Sscanf(parts[1], "%d", &height)
	return
}

// resolveOpenAIKey returns the provided key if non-empty, otherwise falls back
// to the OPENAI_API_KEY environment variable.
func resolveOpenAIKey(configured string) string {
	if configured != "" {
		return configured
	}
	return os.Getenv("OPENAI_API_KEY")
}

// resolveFALKey returns the provided key if non-empty, otherwise falls back
// to the FAL_KEY environment variable.
func resolveFALKey(configured string) string {
	if configured != "" {
		return configured
	}
	return os.Getenv("FAL_KEY")
}
