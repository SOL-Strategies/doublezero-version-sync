#!/bin/bash

# Mock the doublezero binary
function update() {
    echo "Updating DoubleZero to $1"
}

function main() {
    case $1 in
        "--package-version")
            shift
            update "$@"
            ;;
        *)
            echo "Usage: $0 --package-version <package-version>"
            exit 1
    esac
}

main "$@"