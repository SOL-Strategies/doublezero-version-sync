package config

import (
	"github.com/sol-strategies/doublezero-version-sync/internal/sync_commands"
)

// Sync represents the version sync configuration
type Sync struct {
	// Commands are the commands to run when there is a version change
	Commands []sync_commands.Command `koanf:"commands"`
}

// Validate validates the sync configuration
func (s *Sync) Validate() error {
	// This function is kept for any other sync-specific validation that might be needed
	return nil
}
