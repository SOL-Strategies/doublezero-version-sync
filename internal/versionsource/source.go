package versionsource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/hashicorp/go-version"
	"github.com/sol-strategies/doublezero-version-sync/internal/constants"
)

const (
	// Cloudsmith API base URL
	cloudsmithAPIBaseURL = "https://api.cloudsmith.io/packages/malbeclabs"
	// Package name to look for
	packageName = "doublezero"
)

// cloudsmithRepoNames maps cluster names to their Cloudsmith repository names
var cloudsmithRepoNames = map[string]string{
	constants.ClusterNameMainnetBeta: "doublezero",
	constants.ClusterNameTestnet:     "doublezero-testnet",
}

// cloudsmithPackage represents the relevant fields from the Cloudsmith API response
type cloudsmithPackage struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Format    string `json:"format"`
	StatusStr string `json:"status_str"`
}

// Source represents a version source for DoubleZero
type Source struct {
	cluster       string
	logger        *log.Logger
	client        *http.Client
	cachedVersion *version.Version // cached parsed version (use .Original() for package string)
}

// New creates a new version source
func New(cluster string) *Source {
	s := &Source{
		cluster: strings.ToLower(cluster),
		logger:  log.WithPrefix("versionsource"),
		client:  &http.Client{Timeout: 30 * time.Second},
	}

	s.logger.Debug("initialized version source", "cluster", s.cluster)
	return s
}

// GetRecommendedVersion gets the recommended DoubleZero version for the cluster
// Fetches from the Cloudsmith API and returns the latest version
// The result is cached for subsequent calls
func (s *Source) GetRecommendedVersion() (*version.Version, error) {
	if s.cachedVersion != nil {
		return s.cachedVersion, nil
	}

	packageVersion, err := s.fetchLatestVersionFromCloudsmith()
	if err != nil {
		return nil, err
	}

	v, err := version.NewVersion(packageVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse recommended version %s: %w", packageVersion, err)
	}

	s.cachedVersion = v
	s.logger.Info("recommended version", "cluster", s.cluster, "version", v.String())
	return v, nil
}

// GetRecommendedPackageVersion gets the recommended package version string for installation
// Returns the format needed for apt/yum install commands (e.g., "0.7.1-1")
// Uses cached value from GetRecommendedVersion if available
func (s *Source) GetRecommendedPackageVersion() (string, error) {
	if s.cachedVersion != nil {
		return s.cachedVersion.Original(), nil
	}

	v, err := s.GetRecommendedVersion()
	if err != nil {
		return "", err
	}
	return v.Original(), nil
}

// fetchLatestVersionFromCloudsmith fetches the latest doublezero package version from Cloudsmith API
func (s *Source) fetchLatestVersionFromCloudsmith() (string, error) {
	repoName, ok := cloudsmithRepoNames[s.cluster]
	if !ok {
		return "", fmt.Errorf("unknown cluster: %s", s.cluster)
	}

	// Build the API URL with query parameters
	// Use ^doublezero$ to match exactly the package name (not doublezero-sentinel, etc.)
	// Note: Packages are uploaded as "any-distro" so we only filter by name and format
	query := fmt.Sprintf("name:^%s$ format:deb", packageName)
	apiURL := fmt.Sprintf("%s/%s/?query=%s", cloudsmithAPIBaseURL, repoName, url.QueryEscape(query))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "doublezero-version-sync/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch from Cloudsmith API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Cloudsmith API returned status %d for %s", resp.StatusCode, apiURL)
	}

	// Parse the JSON response
	var packages []cloudsmithPackage
	if err := json.NewDecoder(resp.Body).Decode(&packages); err != nil {
		return "", fmt.Errorf("failed to parse Cloudsmith API response: %w", err)
	}

	// Filter for completed deb packages with the correct name
	var versions []string
	for _, pkg := range packages {
		if pkg.Name == packageName && pkg.Format == "deb" && pkg.StatusStr == "Completed" {
			versions = append(versions, pkg.Version)
		}
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("no completed deb packages found for %s in cluster %s", packageName, s.cluster)
	}

	// Sort versions and return the latest
	latestVersion := s.findLatestVersion(versions)
	s.logger.Debug("found latest version from Cloudsmith API", "cluster", s.cluster, "version", latestVersion, "totalVersions", len(versions))

	return latestVersion, nil
}

// findLatestVersion finds the latest version from a list of version strings
// Uses semantic versioning comparison via hashicorp/go-version
func (s *Source) findLatestVersion(versionStrings []string) string {
	if len(versionStrings) == 0 {
		return ""
	}

	type versionEntry struct {
		original string
		parsed   *version.Version
	}

	var entries []versionEntry
	for _, vs := range versionStrings {
		v, err := version.NewVersion(vs)
		if err != nil {
			s.logger.Debug("skipping unparseable version", "version", vs, "error", err)
			continue
		}
		entries = append(entries, versionEntry{original: vs, parsed: v})
	}

	if len(entries) == 0 {
		// Fallback: return the last one in the list if none could be parsed
		return versionStrings[len(versionStrings)-1]
	}

	// Sort by parsed version
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].parsed.LessThan(entries[j].parsed)
	})

	// Return the original string of the latest version
	return entries[len(entries)-1].original
}
