#!/usr/bin/env bash
# Writes the milestone's full diff, new files included, to review/milestone.diff
# for the spec-reviewer subagent (IMPLEMENTATION_PLAN.md, section 2, step 5).
#
# Usage: scripts/review-diff.sh <start-commit>
#
# Plain `git diff` leaves out untracked files, so this stages everything into
# a temporary copy of git's index (GIT_INDEX_FILE) and diffs that against the
# start commit. The developer's own index is untouched. Gitignored files,
# including review/ itself, are left out.
set -euo pipefail

if [ $# -ne 1 ]; then
	echo "usage: $0 <start-commit>" >&2
	exit 2
fi
start="$1"

cd "$(git rev-parse --show-toplevel)"
if ! git rev-parse --verify --quiet "${start}^{commit}" >/dev/null; then
	echo "review-diff: $start is not a commit" >&2
	exit 2
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
index="$(git rev-parse --git-path index)"
if [ -f "$index" ]; then
	cp "$index" "$tmp/index"
fi

export GIT_INDEX_FILE="$tmp/index"
git add -A
mkdir -p review
git diff --cached --find-renames "$start" >review/milestone.diff
unset GIT_INDEX_FILE

files="$(grep -c '^diff --git ' review/milestone.diff || true)"
echo "review-diff: wrote review/milestone.diff ($files files changed since $start)"
