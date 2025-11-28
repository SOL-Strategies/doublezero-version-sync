package doublezero

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/hashicorp/go-version"
	"github.com/sol-strategies/doublezero-version-sync/internal/config"
	"github.com/sol-strategies/doublezero-version-sync/internal/rpc"
	"github.com/sol-strategies/doublezero-version-sync/internal/sync_commands"
	"github.com/sol-strategies/doublezero-version-sync/internal/versiondiff"
	"github.com/sol-strategies/doublezero-version-sync/internal/versionsource"
)

var (
	// versionPattern extracts version strings like "0.7.1", "0.7.1-1", etc.
	// Handles formats like "DoubleZero 0.7.1", "0.7.1-1", etc.
	versionPattern = regexp.MustCompile(`(\d+\.\d+\.\d+(?:-\d+)?)`)
)

// Options represents the options for creating a new DoubleZero instance
type Options struct {
	Cluster          string
	SyncConfig       config.Sync
	DoubleZeroConfig config.DoubleZero
	ValidatorConfig  config.Validator
}

// DoubleZero represents the DoubleZero instance - its state can be refreshed with the RefreshState method
type DoubleZero struct {
	State State

	syncConfig         config.Sync
	logger             *log.Logger
	versionSource      *versionsource.Source
	validatorConfig    config.Validator
	doubleZeroConfig   config.DoubleZero
	validatorRPCClient *rpc.Client
	bin                string
}

// State represents the state of the DoubleZero installation
type State struct {
	Cluster       string
	VersionString string
	Version       *version.Version
}

// New creates a new DoubleZero instance
func New(opts Options) (dz *DoubleZero, err error) {
	bin := opts.DoubleZeroConfig.Bin
	if bin == "" {
		bin = "doublezero"
	}

	dz = &DoubleZero{
		State: State{
			Cluster: opts.Cluster,
		},
		syncConfig:       opts.SyncConfig,
		logger:           log.WithPrefix("doublezero"),
		validatorConfig:  opts.ValidatorConfig,
		doubleZeroConfig: opts.DoubleZeroConfig,
		versionSource:    versionsource.New(opts.Cluster),
		bin:              bin,
	}

	// Set up RPC client if validator is configured (both RPC URL and identity keypairs must be loaded)
	if opts.ValidatorConfig.RPCURL != "" && opts.ValidatorConfig.Identities.ActiveKeyPair != nil && opts.ValidatorConfig.Identities.PassiveKeyPair != nil {
		dz.validatorRPCClient = rpc.NewClient(opts.ValidatorConfig.RPCURL)
	}

	// Parse commands after copying the config
	for i := range dz.syncConfig.Commands {
		err = dz.syncConfig.Commands[i].Parse()
		if err != nil {
			return nil, fmt.Errorf("failed to parse command %d (%s): %w", i, dz.syncConfig.Commands[i].Name, err)
		}
	}

	return dz, nil
}

// SyncVersion syncs the DoubleZero version
func (dz *DoubleZero) SyncVersion() (err error) {
	// refresh the DoubleZero state
	err = dz.refreshState()
	if err != nil {
		return err
	}

	syncLogger := log.WithPrefix("sync").With(
		"cluster", dz.State.Cluster,
	)

	// set a version we'll target as part of a diff
	syncLogger.Debug("creating version diff", "from", dz.State.Version, "fromString", dz.State.VersionString)
	versionDiff := versiondiff.VersionDiff{
		From: dz.State.Version,
	}

	// get the recommended version for the cluster
	versionDiff.To, err = dz.versionSource.GetRecommendedVersion()
	if err != nil {
		return err
	}

	syncLogger.Debug("recommended version from source", "version", versionDiff.To.String())

	// get the package version string for installation commands
	packageVersion, err := dz.versionSource.GetRecommendedPackageVersion()
	if err != nil {
		return err
	}

	syncLogger.Debugf("final target sync version: %s", versionDiff.To.Core().String())
	syncLogger = syncLogger.With("targetVersion", versionDiff.To.Core().String())

	// Check if validator is configured and verify its identity
	if dz.validatorRPCClient != nil {
		if err := dz.checkValidatorIdentity(syncLogger); err != nil {
			return err
		}
	}

	// Check version constraint if configured
	if dz.doubleZeroConfig.VersionConstraint != "" {
		if !dz.doubleZeroConfig.ParsedVersionConstraint.Check(versionDiff.To.Core()) {
			return fmt.Errorf("target version %s does not satisfy doublezero.version_constraint %s", versionDiff.To.Core().String(), dz.doubleZeroConfig.ParsedVersionConstraint.String())
		}
		syncLogger.Debug("target version satisfies version constraint", "constraint", dz.doubleZeroConfig.ParsedVersionConstraint.String())
	}

	// if already on the target version, do nothing
	if versionDiff.IsSameVersion() {
		syncLogger.Info("DoubleZero already running target version - nothing to do")
		return nil
	}

	// by now we know we need to sync
	syncLogger = syncLogger.With("syncDirection", versionDiff.Direction())
	syncLogger.Info(
		fmt.Sprintf("%v  %s required v%s -> v%s",
			versionDiff.DirectionEmoji(), versionDiff.Direction(),
			versionDiff.From.Core().String(), versionDiff.To.Core().String(),
		),
	)

	commandsCount := len(dz.syncConfig.Commands)
	if commandsCount == 0 {
		syncLogger.Warn("no configured commands to execute - skipping")
		return nil
	}

	// create the commands
	syncLogger.Infof("executing commands")
	for cmd_i, cmd := range dz.syncConfig.Commands {
		err := cmd.ExecuteWithData(sync_commands.CommandTemplateData{
			CommandIndex:     cmd_i,
			CommandsCount:    commandsCount,
			ClusterName:      dz.State.Cluster,
			VersionFrom:      versionDiff.From.Core().String(),
			VersionTo:        versionDiff.To.Core().String(),
			PackageVersionTo: packageVersion,
		})
		if err != nil {
			return err
		}
	}

	syncLogger.Infof("commands executed successfully")
	return nil
}

// refreshState refreshes the DoubleZero state
func (dz *DoubleZero) refreshState() error {
	dz.logger.Debug("refreshing DoubleZero state")

	// get the installed version from the binary
	installedVersion, err := dz.getInstalledVersion()
	if err != nil {
		return fmt.Errorf("failed to get installed DoubleZero version: %w", err)
	}
	dz.State.Version = installedVersion
	dz.State.VersionString = installedVersion.String()

	dz.logger.Debug("DoubleZero state refreshed", "version", dz.State.VersionString)

	return nil
}

// getInstalledVersion gets the currently installed DoubleZero version from the configured binary
// The binary is the source of truth for the installed version
// Executes the binary with --version flag and parses the output
func (dz *DoubleZero) getInstalledVersion() (*version.Version, error) {
	cmd := exec.Command(dz.bin, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("bin command failed: %w", err)
	}

	// Parse the output - look for version patterns
	outputStr := strings.TrimSpace(string(output))
	matches := versionPattern.FindStringSubmatch(outputStr)
	if len(matches) > 1 {
		versionStr := matches[1]
		v, err := version.NewVersion(versionStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse version from bin output: %w", err)
		}
		dz.logger.Debug("found installed version from bin", "bin", dz.bin, "version", v.String(), "output", outputStr)
		return v, nil
	}

	return nil, fmt.Errorf("could not extract version from bin output: %s", outputStr)
}

// checkValidatorIdentity checks the validator's identity and ensures sync is allowed
// Returns an error if validator is running with unknown identity or active identity (unless enabled)
func (dz *DoubleZero) checkValidatorIdentity(logger *log.Logger) error {
	validatorIdentity, err := dz.validatorRPCClient.GetIdentity()
	if err != nil {
		return fmt.Errorf("failed to get validator identity: %w", err)
	}

	activeIdentityPK := dz.validatorConfig.Identities.ActiveKeyPair.PublicKey().String()
	passiveIdentityPK := dz.validatorConfig.Identities.PassiveKeyPair.PublicKey().String()

	// Check if validator is running with unknown identity
	if dz.isValidatorUnknown(validatorIdentity, activeIdentityPK, passiveIdentityPK) {
		return fmt.Errorf("validator identity %s does not match configured active (%s) or passive (%s) identities", validatorIdentity, activeIdentityPK, passiveIdentityPK)
	}

	// Check if validator is running as active identity and enabled_when_active is false - sync not allowed
	if dz.isValidatorActive(validatorIdentity, activeIdentityPK) && !dz.validatorConfig.EnabledWhenActive {
		logger.Warnf("validator is running as active identity and we don't run with scissors 🏃✂️")
		return fmt.Errorf("sync not allowed when validator is active (set validator.enabled_when_active=true to allow)")
	}

	// Check if validator is running as active identity and enabled_when_active is true - sync allowed
	if dz.isValidatorActive(validatorIdentity, activeIdentityPK) && dz.validatorConfig.EnabledWhenActive {
		logger.Warn("validator is running as active identity - proceeding with sync (enabled_when_active=true)")
		return nil
	}

	// Validator is running as passive identity
	if dz.isValidatorPassive(validatorIdentity, passiveIdentityPK) {
		logger.Info("validator is running as passive identity - proceeding with sync")
		return nil
	}

	// This should never happen, but handle it just in case
	return fmt.Errorf("unexpected validator identity state")
}

// isValidatorActive returns true if the validator is running as the active identity
func (dz *DoubleZero) isValidatorActive(validatorIdentity, activeIdentityPK string) bool {
	return validatorIdentity == activeIdentityPK
}

// isValidatorPassive returns true if the validator is running as the passive identity
func (dz *DoubleZero) isValidatorPassive(validatorIdentity, passiveIdentityPK string) bool {
	return validatorIdentity == passiveIdentityPK
}

// isValidatorUnknown returns true if the validator is running with an unknown identity
func (dz *DoubleZero) isValidatorUnknown(validatorIdentity, activeIdentityPK, passiveIdentityPK string) bool {
	return validatorIdentity != activeIdentityPK && validatorIdentity != passiveIdentityPK
}
