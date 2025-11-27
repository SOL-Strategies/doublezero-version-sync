package config

import (
	"fmt"

	"github.com/hashicorp/go-version"
)

// DoubleZero represents the DoubleZero configuration
type DoubleZero struct {
	// Bin is the binary/command to use for checking the installed version
	// If not specified, defaults to "doublezero"
	// Examples: "./scripts/mock-doublezero.sh", "doublezero", "/usr/bin/doublezero"
	Bin string `koanf:"bin"`
	// VersionConstraint is the constraint for the DoubleZero version
	// Example: ">= 0.6.9, < 7.0.0"
	VersionConstraint string `koanf:"version_constraint"`
	// ParsedVersionConstraint is the parsed version constraint
	ParsedVersionConstraint version.Constraints `koanf:"-"`
}

// Validate validates the DoubleZero configuration
func (d *DoubleZero) Validate() error {
	// Parse version constraint if provided
	if d.VersionConstraint != "" {
		parsedConstraint, err := version.NewConstraint(d.VersionConstraint)
		if err != nil {
			return fmt.Errorf("failed to parse doublezero.version_constraint: %w", err)
		}
		d.ParsedVersionConstraint = parsedConstraint
	}
	return nil
}
