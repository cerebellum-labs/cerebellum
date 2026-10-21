#!/usr/bin/env bash
# Prints the body of one version's section of a Keep a Changelog file, for
# release notes, and fails when the file has no such section. It is also the
# section check of make release and release-check.sh, so all three agree on
# what a section is. Usage: changelog-section.sh <CHANGELOG.md> <version|Unreleased>
set -euo pipefail

file=$1
version=$2

# A section starts at a "## [version]" heading at the start of a line and ends
# at the next version heading or at the link references at the bottom; blank
# lines around the body are dropped.
awk -v heading="## [$version]" -v file="$file" '
	index($0, heading) == 1 { on = 1; found = 1; next }
	on && (/^## \[/ || /^\[[^]]+\]: /) { exit }
	on { lines[++n] = $0 }
	END {
		if (!found) {
			print file " has no " heading " section" > "/dev/stderr"
			exit 1
		}
		first = 1
		while (first <= n && lines[first] == "") first++
		while (n >= first && lines[n] == "") n--
		for (i = first; i <= n; i++) print lines[i]
	}
' "$file"
