package versionsource

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/hashicorp/go-version"
	"golang.org/x/net/html"
)

const (
	docsURL = "https://docs.malbeclabs.com/setup/#1-install-doublezero-packages"
)

var (
	// debianVersionPattern matches apt-get install commands with doublezero package version
	// Format: apt-get install doublezero=X.Y.Z-N (handles HTML tags and whitespace)
	// We look for this pattern and assume first match is Mainnet-Beta, second is Testnet
	debianVersionPattern = regexp.MustCompile(`apt-get\s+install\s+doublezero\s*=\s*(\d+\.\d+\.\d+-\d+)`)
)

// Source represents a version source for DoubleZero
type Source struct {
	cluster string
	logger  *log.Logger
	client  *http.Client
}

// New creates a new version source
func New(cluster string) *Source {
	return &Source{
		cluster: strings.ToLower(cluster),
		logger:  log.WithPrefix("versionsource"),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// GetRecommendedVersion gets the recommended DoubleZero version for the cluster
// Fetches from the DoubleZero documentation page and extracts the semver part
// (e.g., "0.7.1" from package version "0.7.1-1")
func (s *Source) GetRecommendedVersion() (*version.Version, error) {
	// Fetch package version from docs (e.g., "0.7.1-1")
	packageVersion, err := s.fetchVersionFromDocs()
	if err != nil {
		return nil, err
	}

	// Extract semver part by removing the Debian revision suffix (e.g., "-1")
	// Split on "-" and take the first part
	parts := strings.Split(packageVersion, "-")
	semverString := parts[0]

	v, err := version.NewVersion(semverString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse recommended version %s (from package version %s): %w", semverString, packageVersion, err)
	}

	s.logger.Info("recommended version", "cluster", s.cluster, "version", v.String())
	return v, nil
}

// GetRecommendedPackageVersion gets the recommended package version string for installation
// Returns the format needed for apt/yum install commands
func (s *Source) GetRecommendedPackageVersion() (string, error) {
	// Fetch from docs
	versionString, err := s.fetchVersionFromDocs()
	if err != nil {
		return "", err
	}

	return versionString, nil
}

// fetchVersionFromDocs fetches the recommended version from the DoubleZero documentation
func (s *Source) fetchVersionFromDocs() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", docsURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "doublezero-version-sync/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch docs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("docs returned status %d", resp.StatusCode)
	}

	// Parse HTML to find version strings for all clusters
	versions, err := s.parseVersionFromHTML(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to parse version from docs: %w", err)
	}

	// Look up the version for this cluster
	versionString, ok := versions[s.cluster]
	if !ok {
		return "", fmt.Errorf("could not find version for cluster %s (available clusters: %v)", s.cluster, getMapKeys(versions))
	}

	return versionString, nil
}

// parseVersionFromHTML parses the HTML to extract DoubleZero versions for all clusters
// Extracts text from code blocks and looks for "apt-get install doublezero=X.Y.Z-N" patterns
// Returns a map where key is cluster name and value is the package version
// The first match is for Mainnet-Beta, the second match is for Testnet
// RHEL users should use {{ .VersionTo }} template variable, Debian users should use {{ .PackageVersionTo }}
func (s *Source) parseVersionFromHTML(body io.Reader) (map[string]string, error) {
	doc, err := html.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Collect all versions found in code blocks, keeping track of order
	// We want the first 2 matches in order (Mainnet-Beta, then Testnet)
	// Even if they're the same version, we need both positions
	var foundVersions []string

	// Traverse HTML nodes to find code blocks and extract versions
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Check if this is a code block
			if n.Data == "code" || (n.Data == "pre" && hasCodeChild(n)) {
				text := extractText(n)
				// Look for "apt-get install doublezero=X.Y.Z-N" pattern in the extracted text
				matches := debianVersionPattern.FindAllStringSubmatch(text, -1)
				for _, match := range matches {
					if len(match) > 1 && len(foundVersions) < 2 {
						version := match[1]
						// Add the first 2 matches in order (even if duplicates)
						foundVersions = append(foundVersions, version)
					}
				}
			}
		}

		// Continue traversing
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)

	if len(foundVersions) == 0 {
		return nil, fmt.Errorf("could not find any apt-get install doublezero=X.Y.Z-N patterns in documentation")
	}

	// Build map: first match (index 0) = Mainnet-Beta, second match (index 1) = Testnet
	versions := make(map[string]string)
	if len(foundVersions) > 0 {
		versions["mainnet-beta"] = foundVersions[0]
	}
	if len(foundVersions) > 1 {
		versions["testnet"] = foundVersions[1]
	}

	s.logger.Debug("parsed versions from docs", "versions", versions, "total_matches", len(foundVersions))
	return versions, nil
}

// getMapKeys returns the keys of a map as a slice (for error messages)
func getMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// hasCodeChild checks if a node has a code child element
func hasCodeChild(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "code" {
			return true
		}
	}
	return false
}

// extractText extracts all text content from an HTML node
func extractText(n *html.Node) string {
	var text strings.Builder
	var extract func(*html.Node)
	extract = func(node *html.Node) {
		if node.Type == html.TextNode {
			text.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(n)
	return text.String()
}
