#!/usr/bin/env bash
# Prints which packages a CI run checks, as name=true|false lines for
# $GITHUB_OUTPUT. Usage: changes.sh <event> <ref>
#
# main and manual runs check everything: main must always end with fresh
# images, and a manual run is asked for on purpose. A branch run compares the
# whole branch with main, not with the previous push: after a cancelled run, a
# push that only touches docs must still check the code that run never
# finished.
set -euo pipefail

event=$1
ref=$2

all() {
	echo "server=true"
	exit 0
}

if [[ $event == workflow_dispatch || $ref == refs/heads/main ]]; then
	all
fi

# A branch without history in common with main can't be narrowed down.
base=$(git merge-base origin/main HEAD) || all
files=$(git diff --name-only "$base" HEAD)

# A change to CI itself checks every package.
if grep -q '^\.github/' <<<"$files"; then
	all
fi

changed() {
	if grep -q "^$1/" <<<"$files"; then echo true; else echo false; fi
}

echo "server=$(changed packages/server)"
