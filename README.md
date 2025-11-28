# doublezero-version-sync
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A simple version synchronization manager for DoubleZero, keeping your installed version in sync with the recommended version for your cluster (testnet/mainnet-beta).

## Installation

### From Source

```bash
git clone https://github.com/sol-strategies/doublezero-version-sync.git
cd doublezero-version-sync
make build
```

### Download pre-built binary

Download the latest release from the [Releases page](https://github.com/sol-strategies/doublezero-version-sync/releases).

## Usage

### Run Once

```bash
doublezero-version-sync --config config.yaml run
```

### Run Continuously

```bash
# can run this command as a systemd service
doublezero-version-sync --config config.yaml run --on-interval 1h
```

## Configuration

Create a configuration file (e.g., `config.yml`) with the following options (see [config.yml](config.yml) for a working example):

```yaml
log:
  level: info  # optional, default: info, one of debug|info|warn|error|fatal
  format: text # optional, default: text, one of text|logfmt|json

validator:
  enabled_when_active: false     # optional, default: false - sync only when validator is passive
  rpc_url: http://localhost:8899 # optional, default: http://localhost:8899
  identities:
    active: /path/to/active-identity.json   # required - path to validator active identity keyfile
    passive: /path/to/passive-identity.json # required - path to validator passive identity

cluster:
  name: mainnet-beta # one of mainnet-beta|testnet

doublezero:
  version_constraint: ">= 0.6.9, < 0.7.2" # required - example version constraint
  bin: /path/to/bin/doublezero            # optional, default: doublezero

sync:
  # Commands to run when there is a version change. They will run in the order they are declared.  
  # cmd, args, and environment values can be template strings and will be interpolated with the following variables:
  #  .ClusterName      cluster the DoubleZero instance is running on (testnet/mainnet-beta)
  #  .CommandIndex     index of the command in the commands array (zero-based)
  #  .CommandsCount    count of commands in the commands array
  #  .VersionFrom      current installed version
  #  .VersionTo        sync target version (semver format, e.g., "0.7.1")
  #  .PackageVersionTo package version string for installation (e.g., "0.7.1-1" for Debian/Ubuntu)
  commands:
    - name: "install-doublezero"                                      # required - vanity name for logging purposes
      allow_failure: false                               # optional, default:false - when true, errors are logged and subsequent commands executed
      stream_output: true                                # optional, default: false - when true, command output streamed
      disabled: false                                    # optional, default: false - when true, command skipped
      cmd: /usr/bin/apt-get                              # required, supports templated string
      args: ["install", "-y", "doublezero={{ .PackageVersionTo }}"] # optional, supports templated strings
      environment:                                       # optional, environment variables to pass to cmd, values support templated strings
        DEBIAN_FRONTEND: noninteractive
    # ...
```

## Development

### Prerequisites

- Go 1.25 or later
- Make

### Local Development

```bash
# Build and run locally
make build
make dev

# Build for all platforms
make build-all

# Run tests
make test

# Clean build artifacts
make clean
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.

