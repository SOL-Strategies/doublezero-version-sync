package config

import (
	"github.com/sol-strategies/doublezero-version-sync/internal/constants"
)

// Cluster represents the DoubleZero cluster configuration
type Cluster struct {
	// Name is the DoubleZero cluster this instance is running on. One of mainnet-beta or testnet
	Name string `koanf:"name"`
}

// Validate validates the cluster configuration
func (c *Cluster) Validate() error {
	return constants.ValidateClusterName(c.Name)
}
