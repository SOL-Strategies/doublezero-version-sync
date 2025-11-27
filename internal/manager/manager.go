package manager

import (
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/sol-strategies/doublezero-version-sync/internal/config"
	"github.com/sol-strategies/doublezero-version-sync/internal/doublezero"
)

// Manager manages the DoubleZero version sync process
type Manager struct {
	cfg        *config.Config
	logger     *log.Logger
	doublezero *doublezero.DoubleZero
}

// NewFromConfig creates a new Manager from an already loaded config
func NewFromConfig(cfg *config.Config) (m *Manager, err error) {
	m = &Manager{
		cfg:    cfg,
		logger: log.WithPrefix("manager"),
	}

	// Create DoubleZero instance
	m.doublezero, err = doublezero.New(doublezero.Options{
		Cluster:          cfg.Cluster.Name,
		SyncConfig:       cfg.Sync,
		DoubleZeroConfig: cfg.DoubleZero,
		ValidatorConfig:  cfg.Validator,
	})

	if err != nil {
		return nil, err
	}

	// manager created
	m.logger.Debug("created manager from config",
		"config", cfg,
		"doublezero_bin", cfg.DoubleZero.Bin,
		"validator_rpc_url", cfg.Validator.RPCURL,
		"validator_has_identities", cfg.Validator.Identities.ActiveKeyPair != nil && cfg.Validator.Identities.PassiveKeyPair != nil)
	return m, nil
}

// RunOnce runs a single sync check and exits
func (m *Manager) RunOnce() error {
	m.logger.Info("🚀 starting doublezero-version-sync (single run mode)")
	return m.doublezero.SyncVersion()
}

// RunOnInterval runs the sync manager continuously at the specified interval, errors are logged but not returned after parsing the interval duration string
func (m *Manager) RunOnInterval(intervalDuration time.Duration) (err error) {
	m.logger.Info("🚀 starting doublezero-version-sync (continuous mode)", "interval", intervalDuration.String())

	// Calculate the next boundary time based on the interval
	now := time.Now().UTC()
	nextSyncTime := m.calculateNextBoundary(now, intervalDuration)

	// Wait until the first boundary before starting
	if nextSyncTime.After(now) {
		waitDuration := nextSyncTime.Sub(now)
		m.logger.Info("waiting until next interval boundary", "wait", waitDuration.String(), "next_sync", nextSyncTime.Format("2006-01-02T15:04:05Z"))
		time.Sleep(waitDuration)
	}

	// Run sync on a loop, aligning to interval boundaries
	for {
		m.runSyncVersionInterval(intervalDuration)

		// Calculate next boundary time
		now = time.Now().UTC()
		nextSyncTime = m.calculateNextBoundary(now, intervalDuration)
		waitDuration := nextSyncTime.Sub(now)

		if waitDuration > 0 {
			time.Sleep(waitDuration)
		}
	}
}

// calculateNextBoundary calculates the next time boundary based on the interval duration
// For example, if interval is 10m and current time is 9:53, it returns 10:00
// Boundaries align with clock times (e.g., for 5m: :00, :05, :10, :15, etc.)
func (m *Manager) calculateNextBoundary(now time.Time, intervalDuration time.Duration) time.Time {
	// Truncate to the start of the day (midnight)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Calculate duration since midnight
	durationSinceMidnight := now.Sub(startOfDay)

	// Truncate to the previous interval boundary
	truncatedDuration := durationSinceMidnight.Truncate(intervalDuration)

	// Add one interval to get the next boundary
	nextBoundaryDuration := truncatedDuration + intervalDuration

	// Calculate the next boundary time
	nextBoundary := startOfDay.Add(nextBoundaryDuration)

	return nextBoundary
}

// runSyncVersionInterval runs the sync version and logs the result without returning an error - used with on interval mode
func (m *Manager) runSyncVersionInterval(intervalDuration time.Duration) {
	m.logger.Info("running sync")
	err := m.doublezero.SyncVersion()
	now := time.Now().UTC()
	nextSyncTime := m.calculateNextBoundary(now, intervalDuration)

	// Set result string
	resultString := "succeeded"
	if err != nil {
		resultString = "failed"
	}

	waitDuration := nextSyncTime.Sub(now)
	msg := fmt.Sprintf("sync %s - next sync in %s at %s",
		resultString, waitDuration.String(), nextSyncTime.Format("2006-01-02T15:04:05Z"),
	)

	if err != nil {
		m.logger.With("error", err).Error(msg)
	} else {
		m.logger.Info(msg)
	}
}
