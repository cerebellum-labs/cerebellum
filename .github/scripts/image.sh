#!/usr/bin/env bash
# Prints the GHCR image of a package: the one place image names are defined,
# so the workflow that pushes an image and the one that releases it can't
# drift apart. Usage: image.sh <package>
set -euo pipefail

echo "ghcr.io/${GITHUB_REPOSITORY_OWNER}/cerebellum-$1"
