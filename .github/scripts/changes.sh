#!/usr/bin/env bash
# Prints what a CI run checks, as name=true|false lines for $GITHUB_OUTPUT:
# each package, and whether the server image is built.
# Usage: changes.sh <event> <ref> [<commit of the current :main image>]
#
# A manual run checks and builds everything: it is asked for on purpose.
#
# A branch run compares the whole branch with main, not with the previous
# push: after a cancelled run, a push that only touches docs must still check
# the code that run never finished. A branch builds the server image only when
# it changes how the image is built (the Dockerfile or .dockerignore): Go code
# is already compiled by the tests, and the image build adds minutes.
#
# On main, the server is compared with the commit its :main image was built
# from, not with the previous commit: GitHub keeps one waiting run, so a merge
# whose run was cancelled in the queue still gets its image from the next one.
# The dashboard publishes nothing from CI, so main checks it against the
# previous commit (one squash-merged pull request).
#
# A change to CI itself, or anything that can't be narrowed down, checks and
# builds everything.
set -euo pipefail

event=$1
ref=$2
image_revision=${3:-}

all() {
	echo "server=true"
	echo "server-image=true"
	echo "dashboard=true"
	exit 0
}

changed() { # changed <regex> <files>
	if grep -Eq "$1" <<<"$2"; then echo true; else echo false; fi
}

# --no-renames throughout: rename detection reads file contents, which a
# blobless checkout doesn't have and can't fetch without credentials. It also
# lists a moved file under both its old and new path, so both packages count.
diff_files() {
	git diff --no-renames --name-only "$1" HEAD
}

if [[ $event == workflow_dispatch ]]; then
	all
fi

if [[ $ref == refs/heads/main ]]; then
	# No image yet, an image without its commit, or a commit main no longer
	# contains: nothing to compare with.
	if [[ -z $image_revision ]] || ! git merge-base --is-ancestor "$image_revision" HEAD 2>/dev/null; then
		all
	fi
	since_image=$(diff_files "$image_revision")
	since_parent=$(diff_files HEAD^) || all
	if grep -q '^\.github/' <<<"$since_image"$'\n'"$since_parent"; then
		all
	fi
	server=$(changed '^packages/server/' "$since_image")
	echo "server=$server"
	# On main the image follows the checks: a changed server is published.
	echo "server-image=$server"
	echo "dashboard=$(changed '^packages/dashboard/' "$since_parent")"
	exit 0
fi

# A branch without history in common with main can't be narrowed down.
base=$(git merge-base origin/main HEAD) || all
files=$(diff_files "$base")

if grep -q '^\.github/' <<<"$files"; then
	all
fi

echo "server=$(changed '^packages/server/' "$files")"
echo "server-image=$(changed '^packages/server/(Dockerfile|\.dockerignore)$' "$files")"
echo "dashboard=$(changed '^packages/dashboard/' "$files")"
