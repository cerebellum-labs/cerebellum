#!/usr/bin/env bash
# Checks a release tag before anything is published and prints its facts as
# key=value lines for $GITHUB_OUTPUT. Usage: release-check.sh <tag> <commit>
#
# Needs the full history and all tags (actions/checkout with fetch-depth: 0).
set -euo pipefail

tag=$1
commit=$2

fail() {
	echo "::error::$tag: $*" >&2
	exit 1
}

package=${tag%%/v*}
version=${tag#*/v}

case $package in
server) ;;
*) fail "unknown package '$package'" ;;
esac

if [[ ! $version =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.(0|[1-9][0-9]*))?$ ]]; then
	fail "version must be X.Y.Z or X.Y.Z-rc.N"
fi
core=${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.${BASH_REMATCH[3]}
minor=${BASH_REMATCH[1]}.${BASH_REMATCH[2]}
prerelease=false
if [[ -n ${BASH_REMATCH[4]} ]]; then
	prerelease=true
fi

# Only code that went through review on main is released.
if ! git merge-base --is-ancestor "$commit" origin/main; then
	fail "commit $commit is not on main"
fi

# Versions only grow: X.Y.Z must be above every earlier final release, so a
# late tag can't move latest or X.Y back to older code. Release candidates
# compare by X.Y.Z, which also keeps them from following their own release.
newest=$(git tag -l "$package/v*" | sed "s|^$package/v||" |
	grep -Ex '[0-9]+\.[0-9]+\.[0-9]+' | grep -vFx "$version" | sort -V | tail -n 1 || true)
if [[ -n $newest && $(printf '%s\n' "$newest" "$core" | sort -V | tail -n 1) == "$newest" ]]; then
	fail "$core is not above the latest release $newest"
fi

if [[ $prerelease == false ]] &&
	! "$(dirname "$0")/changelog-section.sh" "packages/$package/CHANGELOG.md" "$version" >/dev/null 2>&1; then
	fail "packages/$package/CHANGELOG.md has no ## [$version] section"
fi

echo "package=$package"
echo "image=$("$(dirname "$0")/image.sh" "$package")"
echo "version=$version"
echo "core=$core"
echo "minor=$minor"
echo "prerelease=$prerelease"
