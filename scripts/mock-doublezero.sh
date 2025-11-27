#!/bin/bash

# Mock the doublezero binary
function version() {
    echo "DoubleZero 0.6.9"
}

function main() {
    case $1 in
        "--version")
            version
            ;;
        *)
            echo "Usage: $0 [version]"
            exit 1
    esac
}

main "$@"