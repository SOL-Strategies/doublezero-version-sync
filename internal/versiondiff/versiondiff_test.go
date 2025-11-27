package versiondiff

import (
	"testing"

	"github.com/hashicorp/go-version"
)

func TestVersionDiff_IsSameVersion(t *testing.T) {
	tests := []struct {
		name     string
		from     *version.Version
		to       *version.Version
		expected bool
	}{
		{
			name:     "same versions",
			from:     version.Must(version.NewVersion("1.0.0")),
			to:       version.Must(version.NewVersion("1.0.0")),
			expected: true,
		},
		{
			name:     "different versions",
			from:     version.Must(version.NewVersion("1.0.0")),
			to:       version.Must(version.NewVersion("1.0.1")),
			expected: false,
		},
		{
			name:     "nil from",
			from:     nil,
			to:       version.Must(version.NewVersion("1.0.0")),
			expected: false,
		},
		{
			name:     "nil to",
			from:     version.Must(version.NewVersion("1.0.0")),
			to:       nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vd := VersionDiff{From: tt.from, To: tt.to}
			if vd.IsSameVersion() != tt.expected {
				t.Errorf("IsSameVersion() = %v, want %v", vd.IsSameVersion(), tt.expected)
			}
		})
	}
}

func TestVersionDiff_Direction(t *testing.T) {
	tests := []struct {
		name     string
		from     *version.Version
		to       *version.Version
		expected string
	}{
		{
			name:     "upgrade",
			from:     version.Must(version.NewVersion("1.0.0")),
			to:       version.Must(version.NewVersion("1.0.1")),
			expected: "upgrade",
		},
		{
			name:     "downgrade",
			from:     version.Must(version.NewVersion("1.0.1")),
			to:       version.Must(version.NewVersion("1.0.0")),
			expected: "downgrade",
		},
		{
			name:     "no change",
			from:     version.Must(version.NewVersion("1.0.0")),
			to:       version.Must(version.NewVersion("1.0.0")),
			expected: "no change",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vd := VersionDiff{From: tt.from, To: tt.to}
			if vd.Direction() != tt.expected {
				t.Errorf("Direction() = %v, want %v", vd.Direction(), tt.expected)
			}
		})
	}
}
