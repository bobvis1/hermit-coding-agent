#!/usr/bin/env bash
# Claude Code Stop hook (IMPLEMENTATION_PLAN.md, milestone 0): the agent may
# not finish while `make check` fails. Two exceptions:
#   - a question is pending: QUESTION.md exists, so the stop is allowed and
#     the question is shown to the developer;
#   - nothing `make check` depends on changed in the working tree.
# It deliberately ignores stop_hook_active: Claude Code itself ends the turn
# after repeated blocked stops (CLAUDE_CODE_STOP_HOOK_BLOCK_CAP).
#
# Exit 0 allows the stop; exit 2 blocks it and gives stderr to the agent.
set -u

cat >/dev/null # the hook's JSON input is not needed

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/json.sh
. "$here/lib/json.sh"

root="${CLAUDE_PROJECT_DIR:-}"
if [ -z "$root" ]; then
	root="$(git rev-parse --show-toplevel 2>/dev/null)" || root=""
fi
if [ -z "$root" ] || ! cd "$root"; then
	echo "stop-check: can't find the repository root, so make check can't run." >&2
	exit 2
fi

if [ -f QUESTION.md ]; then
	printf '{"systemMessage": '
	{
		echo "The agent stopped to ask a question (QUESTION.md). Answer it, delete QUESTION.md, then resume the session."
		echo
		cat QUESTION.md
	} | json_string
	printf '}\n'
	exit 0
fi

# Does a file that `make check` depends on differ from HEAD, or is one new?
# In -z output a rename or copy is followed by a record holding the old path.
statusfile="$(mktemp)"
trap 'rm -f "$statusfile"' EXIT
if ! git status --porcelain=v1 -z --untracked-files=all >"$statusfile" 2>/dev/null; then
	echo "stop-check: git status failed, so changes can't be checked." >&2
	exit 2
fi
relevant=""
old_path_next=0
while IFS= read -r -d '' record; do
	if [ "$old_path_next" = 1 ]; then
		path="$record"
		old_path_next=0
	else
		path="${record:3}"
		case "${record:0:2}" in R* | C* | ?R | ?C) old_path_next=1 ;; esac
	fi
	case "$path" in
	*.go | go.mod | go.sum | */go.mod | */go.sum | Makefile | testdata/* | */testdata/* | scripts/* | deps/* | .github/*)
		relevant="$path"
		break
		;;
	esac
done <"$statusfile"

if [ -z "$relevant" ]; then
	exit 0
fi

if out="$(make check 2>&1)"; then
	exit 0
fi
{
	echo "make check fails (changed: $relevant and possibly others), so this milestone isn't done."
	echo "Fix the failures below and run make check again. If the requirements are wrong,"
	echo "ambiguous or contradictory, write the question to QUESTION.md and stop instead."
	echo
	printf '%s\n' "$out" | tail -n 80
} >&2
exit 2
