#!/usr/bin/env bash
# Prints what a CI run checks, as name=true|false lines for $GITHUB_OUTPUT:
# each package, and whether its image is built. Usage: changes.sh <event> <ref>
#
# main and manual runs check and build everything: main must always end with
# fresh images, and a manual run is asked for on purpose. A branch run
# compares the whole branch with main, not with the previous push: after a
# cancelled run, a push that only touches docs must still check the code that
# run never finished.
#
# A branch builds an image only when it changes how the image is built (the
# Dockerfile, .dockerignore or CI): Go code is already compiled by the tests,
# and the image build would add minutes to every push.
set -euo pipefail

event=$1
ref=$2

all() {
	echo "server=true"
	echo "server-image=true"
	echo "dashboard=true"
	exit 0
}

if [[ $event == workflow_dispatch || $ref == refs/heads/main ]]; then
	all
fi

# A branch without history in common with main can't be narrowed down.
base=$(git merge-base origin/main HEAD) || all
# --no-renames: rename detection reads file contents, which a blobless
# checkout doesn't have and can't fetch without credentials. It also lists a
# moved file under both its old and new path, so both packages count.
files=$(git diff --no-renames --name-only "$base" HEAD)

# A change to CI itself checks and builds everything.
if grep -q '^\.github/' <<<"$files"; then
	all
fi

changed() {
	if grep -Eq "$1" <<<"$files"; then echo true; else echo false; fi
}

echo "server=$(changed '^packages/server/')"
echo "server-image=$(changed '^packages/server/(Dockerfile|\.dockerignore)$')"
echo "dashboard=$(changed '^packages/dashboard/')"
