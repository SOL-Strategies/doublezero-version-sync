package constants

import (
	"fmt"
	"slices"
	"strings"
)

const (
	// ClusterNameMainnetBeta is the name of the Mainnet Beta cluster
	ClusterNameMainnetBeta = "mainnet-beta"
	// ClusterNameTestnet is the name of the Testnet cluster
	ClusterNameTestnet = "testnet"
)

// ValidClusterNames is a list of valid cluster names
var ValidClusterNames = []string{ClusterNameMainnetBeta, ClusterNameTestnet}

// ValidateClusterName validates a cluster name
func ValidateClusterName(clusterName string) (err error) {
	if !slices.Contains(ValidClusterNames, clusterName) {
		return fmt.Errorf("invalid cluster name: %s - must be one of %s", clusterName, strings.Join(ValidClusterNames, ", "))
	}
	return nil
}
