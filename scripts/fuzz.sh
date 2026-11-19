#!/usr/bin/env bash
# Runs every fuzz target in the hermit module for the given time each.
# Usage: scripts/fuzz.sh [fuzztime]   (default 60s)
set -euo pipefail

fuzztime="${1:-60s}"
go="${GO:-go}"
count=0
for pkg in $("$go" list ./...); do
	for target in $("$go" test -list '^Fuzz' "$pkg" | grep '^Fuzz' || true); do
		echo "fuzz: $pkg $target ($fuzztime)"
		"$go" test -run '^$' -fuzz "^${target}\$" -fuzztime "$fuzztime" "$pkg"
		count=$((count + 1))
	done
done
echo "fuzz: ran $count target(s)"
