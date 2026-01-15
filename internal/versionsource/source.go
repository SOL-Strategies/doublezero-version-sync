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

	// Parse HTML to find version strings
	// TODO: This is pretty hacky, alternative could be to piece together the latest version for the running os,distro and arch like this:
	// curl -s "https://dl.cloudsmith.io/public/malbeclabs/doublezero/deb/ubuntu/dists/jammy/main/binary-amd64/Packages.gz" | gunzip | grep -A 1 "^Package: doublezero$" | grep "^Version:" | sort -V | tail -1 | cut -d' ' -f2
	// the problem here is that it doesn't differentiate between mainnet and testnet or whether it is recommended.
	versionString, err := s.parseVersionFromHTML(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to parse version from docs: %w", err)
	}

	return versionString, nil
}

// parseVersionFromHTML parses the HTML to extract the DoubleZero version for the configured cluster
// Extracts text from code blocks and looks for "doublezero=X.Y.Z-N" patterns
// The first match is for Mainnet-Beta, the second match is for Testnet
// RHEL users should use {{ .VersionTo }} template variable, Debian users should use {{ .PackageVersionTo }}
func (s *Source) parseVersionFromHTML(body io.Reader) (string, error) {
	doc, err := html.Parse(body)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Collect all versions found in code blocks, keeping track of order
	var foundVersions []string
	seen := make(map[string]bool)

	// Traverse HTML nodes to find code blocks and extract versions
	// We only want the first 2 unique Debian/Ubuntu versions (Mainnet-Beta, then Testnet)
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Check if this is a code block
			if n.Data == "code" || (n.Data == "pre" && hasCodeChild(n)) {
				text := extractText(n)
				// Look for "apt-get install doublezero=X.Y.Z-N" pattern in the extracted text
				matches := debianVersionPattern.FindAllStringSubmatch(text, -1)
				for _, match := range matches {
					if len(match) > 1 {
						version := match[1]
						// Only add if we haven't seen it and we don't have 2 yet
						if !seen[version] && len(foundVersions) < 2 {
							foundVersions = append(foundVersions, version)
							seen[version] = true
						}
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
		return "", fmt.Errorf("could not find any apt-get install doublezero=X.Y.Z-N patterns in documentation")
	}

	// Determine which match to use based on cluster
	// First match (index 0) = Mainnet-Beta
	// Second match (index 1) = Testnet
	var matchIndex int
	switch s.cluster {
	case "mainnet-beta":
		matchIndex = 0
	case "testnet":
		matchIndex = 1
	default:
		return "", fmt.Errorf("unknown cluster: %s", s.cluster)
	}

	if matchIndex >= len(foundVersions) {
		return "", fmt.Errorf("could not find version for cluster %s (found %d matches, need index %d)", s.cluster, len(foundVersions), matchIndex)
	}

	foundVersion := foundVersions[matchIndex]

	s.logger.Debug("parsed version from docs", "cluster", s.cluster, "version", foundVersion, "match_index", matchIndex, "total_matches", len(foundVersions))
	return foundVersion, nil
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
