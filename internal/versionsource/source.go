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
	// debianPattern matches Debian/Ubuntu package version format: doublezero=0.7.1-1
	debianPattern = regexp.MustCompile(`doublezero=(\d+\.\d+\.\d+-\d+)`)
)

// Source represents a version source for DoubleZero
type Source struct {
	cluster           string
	logger            *log.Logger
	client            *http.Client
	deploymentPattern *regexp.Regexp
}

// New creates a new version source
func New(cluster string) *Source {
	// Build the deployment pattern: "the current recommended deployment for <cluster> is:"
	// Convert cluster name to lowercase for case-insensitive matching
	clusterLower := strings.ToLower(cluster)
	// Compile the pattern once at creation time
	deploymentPattern := regexp.MustCompile(fmt.Sprintf(`the current recommended deployment for %s is:`, clusterLower))

	return &Source{
		cluster:           clusterLower,
		logger:            log.WithPrefix("versionsource"),
		client:            &http.Client{Timeout: 30 * time.Second},
		deploymentPattern: deploymentPattern,
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
// Looks for the text pattern "The current recommended deployment for <cluster> is:" (case-insensitive)
// Once found, extracts the version from the Debian/Ubuntu code block that follows it
// RHEL users should use {{ .VersionTo }} template variable, Debian users should use {{ .PackageVersionTo }}
func (s *Source) parseVersionFromHTML(body io.Reader) (string, error) {
	doc, err := html.Parse(body)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	var foundVersion string

	// Traverse HTML nodes to find the deployment text, then extract version from following code block
	// Track whether we've found the deployment text for our cluster
	var foundDeploymentText bool
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		// Extract text from this node and its children for pattern matching
		nodeText := extractText(n)
		nodeTextLower := strings.ToLower(nodeText)

		// Check if this node contains the deployment text pattern
		if !foundDeploymentText {
			if s.deploymentPattern.MatchString(nodeTextLower) {
				foundDeploymentText = true
				// Continue traversing to find the code block that follows
			}
		}

		// If we've found the deployment text, look for the next code block
		if foundDeploymentText && foundVersion == "" {
			if n.Type == html.ElementNode {
				// Check if this is a code block
				if n.Data == "code" || (n.Data == "pre" && hasCodeChild(n)) {
					text := extractText(n)
					// Look for Debian/Ubuntu package version format
					if matches := debianPattern.FindStringSubmatch(text); len(matches) > 1 {
						foundVersion = matches[1]
						return // Found the version, stop searching
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

	if foundVersion == "" {
		return "", fmt.Errorf("could not find Debian/Ubuntu package version string (doublezero=X.Y.Z-N) in documentation for cluster %s", s.cluster)
	}

	s.logger.Debug("parsed version from docs", "cluster", s.cluster, "version", foundVersion)
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
