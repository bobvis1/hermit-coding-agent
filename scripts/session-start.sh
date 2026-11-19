#!/usr/bin/env bash
# Claude Code SessionStart hook for startup and resume (IMPLEMENTATION_PLAN.md,
# milestone 0). A QUESTION.md lets the Stop hook allow a stop with failing
# checks, so one left over from an earlier session is renamed here: a
# QUESTION.md the Stop hook sees was always written in the current session.
set -u

cat >/dev/null

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/json.sh
. "$here/lib/json.sh"

root="${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null)}"
cd "$root" 2>/dev/null || exit 0
[ -f QUESTION.md ] || exit 0

dest="QUESTION.unanswered-$(date +%Y-%m-%dT%H%M%S).md"
n=1
while [ -e "$dest" ]; do
	dest="QUESTION.unanswered-$(date +%Y-%m-%dT%H%M%S)-$n.md"
	n=$((n + 1))
done
if ! mv QUESTION.md "$dest"; then
	echo "session-start: couldn't rename QUESTION.md; delete it before working, or the Stop hook will let failing checks through." >&2
	exit 2
fi

msg="An unanswered QUESTION.md from an earlier session was renamed to $dest. Answer it or delete it; it no longer lets this session stop with failing checks."
printf '{"systemMessage": %s, "hookSpecificOutput": {"hookEventName": "SessionStart", "additionalContext": %s}}\n' \
	"$(printf '%s' "$msg" | json_string)" "$(printf '%s' "$msg" | json_string)"
exit 0
