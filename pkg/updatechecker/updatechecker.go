package updatechecker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/go-version"
)

const (
	githubAPIURL = "https://api.github.com/repos/google-gemini/gemini-cli/releases/latest"
)

var (
	// CurrentVersion is the current version of the CLI.
	// This is intended to be set at build time.
	CurrentVersion = "0.0.1"
)

// ReleaseInfo contains the information about a new release.
type ReleaseInfo struct {
	Version     string    `json:"tag_name"`
	URL         string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
}

// CheckForUpdates checks for new releases on GitHub.
func CheckForUpdates() (*ReleaseInfo, error) {
	resp, err := http.Get(githubAPIURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch releases: status code %d", resp.StatusCode)
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode release info: %w", err)
	}

	latestVersion, err := version.NewVersion(release.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to parse latest version: %w", err)
	}

	v, err := version.NewVersion(CurrentVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse current version: %w", err)
	}

	if latestVersion.GreaterThan(v) {
		return &release, nil
	}

	return nil, nil
}