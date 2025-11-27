package versiondiff

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/hashicorp/go-version"
)

var (
	upArrowStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("28")).Bold(true)
	downArrowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("124")).Bold(true)
	sameStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// VersionDiff represents a version difference
type VersionDiff struct {
	From *version.Version
	To   *version.Version
}

// IsSameVersion checks if the versions are the same
func (v VersionDiff) IsSameVersion() bool {
	if v.From == nil || v.To == nil {
		return false
	}
	return v.From.Core().Compare(v.To.Core()) == 0
}

// Direction returns the direction of the version change
func (v VersionDiff) Direction() string {
	if v.IsSameVersion() {
		return "no change"
	}
	if v.To.Core().GreaterThan(v.From.Core()) {
		return "upgrade"
	}
	return "downgrade"
}

// DirectionEmoji returns an emoji representing the direction of the version change
func (v VersionDiff) DirectionEmoji() string {
	if v.IsSameVersion() {
		return sameStyle.Render("=")
	}
	if v.To.Core().GreaterThan(v.From.Core()) {
		return upArrowStyle.Render("↑")
	}
	return downArrowStyle.Render("↓")
}

// String returns a string representation of the version diff
func (v VersionDiff) String() string {
	if v.From == nil {
		return fmt.Sprintf("unknown -> %s", v.To.Core().String())
	}
	if v.To == nil {
		return fmt.Sprintf("%s -> unknown", v.From.Core().String())
	}
	return fmt.Sprintf("%s -> %s", v.From.Core().String(), v.To.Core().String())
}
