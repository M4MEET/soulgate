package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// currentVersion must match rootCmd.Version so the comparison is always accurate.
const currentVersion = "0.2.0"

// githubRepo is the canonical repository for release queries.
const githubRepo = "M4MEET/soulgate"

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update SoulGate to the latest version",
	Long: `Check GitHub releases and download the latest SoulGate binary.

The command fetches the latest release from GitHub, compares it with the
running version, and replaces the current binary atomically if a newer
version is available.

Example:
  soulgate update
  soulgate update --check`,
	Args: cobra.NoArgs,
	RunE: runUpdate,
}

var updateCheckOnly bool

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVar(&updateCheckOnly, "check", false, "Only check for updates; do not download")
}

// githubRelease is the subset of the GitHub releases API response we need.
type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func runUpdate(cmd *cobra.Command, args []string) error {
	fmt.Printf("Current version: v%s\n", currentVersion)
	fmt.Println("Checking for updates...")

	release, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to fetch release info: %w", err)
	}

	latestTag := strings.TrimPrefix(release.TagName, "v")
	fmt.Printf("Latest version:  %s\n", release.TagName)

	if latestTag == currentVersion {
		fmt.Println("Already up to date.")
		return nil
	}

	// Simple lexicographic version check — good enough for semver tags like
	// v0.2.0, v0.3.0, v1.0.0. A proper semver library is not imported to keep
	// the update command dependency-free.
	if latestTag <= currentVersion {
		fmt.Printf("Current version v%s is newer than or equal to the latest release %s.\n",
			currentVersion, release.TagName)
		return nil
	}

	if updateCheckOnly {
		fmt.Printf("Update available: v%s -> %s\n", currentVersion, release.TagName)
		fmt.Println("Run 'soulgate update' (without --check) to install.")
		return nil
	}

	// Determine the expected asset name for this platform.
	assetName := binaryAssetName()
	downloadURL := ""
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf(
			"no binary found for %s/%s in release %s (expected asset: %s)",
			runtime.GOOS, runtime.GOARCH, release.TagName, assetName,
		)
	}

	fmt.Printf("Downloading %s...\n", assetName)

	// Download into a temp file alongside the current binary.
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine current binary path: %w", err)
	}
	selfPath, err = filepath.EvalSymlinks(selfPath)
	if err != nil {
		return fmt.Errorf("could not resolve binary symlink: %w", err)
	}

	dir := filepath.Dir(selfPath)
	tmpFile, err := os.CreateTemp(dir, ".soulgate-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file for download: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		// Clean up temp file on any error path; ignore if already renamed.
		os.Remove(tmpPath) //nolint:errcheck
	}()

	if err := downloadFile(tmpFile, downloadURL); err != nil {
		tmpFile.Close()
		return fmt.Errorf("download failed: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to flush download: %w", err)
	}

	// Make the downloaded file executable.
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed to chmod downloaded binary: %w", err)
	}

	// Atomic rename: replace the running binary.
	if err := os.Rename(tmpPath, selfPath); err != nil {
		return fmt.Errorf("failed to replace binary (try running as root): %w", err)
	}

	fmt.Printf("Updated from v%s to %s\n", currentVersion, release.TagName)
	return nil
}

// fetchLatestRelease queries the GitHub releases API for the latest release.
func fetchLatestRelease() (*githubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "soulgate/"+currentVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("repository %s has no releases yet", githubRepo)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if release.TagName == "" {
		return nil, fmt.Errorf("release response contained no tag_name")
	}

	return &release, nil
}

// downloadFile streams the response body from url into dst.
func downloadFile(dst io.Writer, url string) error {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned HTTP %d", resp.StatusCode)
	}

	if _, err := io.Copy(dst, resp.Body); err != nil {
		return fmt.Errorf("copy body: %w", err)
	}
	return nil
}

// binaryAssetName returns the expected release asset filename for the current
// OS and architecture, following the pattern "soulgate_{os}_{arch}".
func binaryAssetName() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	name := fmt.Sprintf("soulgate_%s_%s", goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name
}
