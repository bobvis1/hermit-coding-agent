#!/usr/bin/env bash
# Tests for the Claude Code hooks and the review script (IMPLEMENTATION_PLAN.md,
# milestone 0 checks). Each test builds a throwaway git repository with its own
# small Makefile, so `make check` inside it is fast and never recurses here.
set -u

scripts="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
failures=0
tmproot="$(mktemp -d)"
trap 'rm -rf "$tmproot"' EXIT

fail() {
	echo "FAIL: $*"
	failures=$((failures + 1))
}
pass() { echo "ok:   $*"; }

# valid_json checks its input is JSON when python3 is available.
valid_json() {
	if command -v python3 >/dev/null 2>&1; then
		python3 -c 'import json,sys; json.load(sys.stdin)'
	else
		cat >/dev/null
	fi
}

# newrepo makes a repository with a committed README, a Go module and a test
# that passes, and a Makefile whose check target runs that test.
newrepo() {
	local dir="$tmproot/$1"
	mkdir -p "$dir/scripts"
	cp -R "$scripts/." "$dir/scripts/"
	(
		cd "$dir" || exit 1
		git init -q
		git config user.email test@example.com
		git config user.name test
		git config commit.gpgsign false
		printf '# readme\n' >README.md
		printf 'module example.com/fixture\n\ngo 1.25.0\n' >go.mod
		printf 'package fixture\n\nimport "testing"\n\nfunc TestOK(t *testing.T) {}\n' >ok_test.go
		printf 'check:\n\tgo test ./...\n' >Makefile
		git add -A
		git commit -qm init
	)
	echo "$dir"
}

stop_hook() { # dir -> runs the Stop hook there; sets $code, $out, $err
	out="$(cd "$1" && echo '{}' | CLAUDE_PROJECT_DIR="$1" bash scripts/stop-check.sh 2>"$tmproot/err")"
	code=$?
	err="$(cat "$tmproot/err")"
}

session_hook() {
	out="$(cd "$1" && echo '{}' | CLAUDE_PROJECT_DIR="$1" bash scripts/session-start.sh 2>"$tmproot/err")"
	code=$?
	err="$(cat "$tmproot/err")"
}

add_failing_test() {
	printf 'package fixture\n\nimport "testing"\n\nfunc TestFails(t *testing.T) { t.Fatal("deliberate failure") }\n' >"$1/fail_test.go"
}

# --- Stop hook ---

d="$(newrepo docs)"
# A failing check is committed, so the stop is allowed only because nothing
# but documentation changed.
(cd "$d" && printf 'check:\n\texit 1\n' >Makefile && git commit -qam "failing check" &&
	printf 'more\n' >>README.md && mkdir docs && printf 'notes\n' >docs/notes.md)
stop_hook "$d"
[ "$code" = 0 ] && pass "Stop hook allows a documentation-only change" || fail "Stop hook blocked a documentation-only change ($code): $err"

d="$(newrepo failing)"
add_failing_test "$d"
stop_hook "$d"
if [ "$code" = 2 ] && printf '%s' "$err" | grep -q "deliberate failure"; then
	pass "Stop hook blocks a deliberately failing test, showing the failure"
else
	fail "Stop hook didn't block a failing test (exit $code): $err"
fi

d="$(newrepo passing)"
printf 'package fixture\n\nfunc Add(a, b int) int { return a + b }\n' >"$d/add.go"
stop_hook "$d"
[ "$code" = 0 ] && pass "Stop hook allows a Go change whose checks pass" || fail "Stop hook blocked passing checks ($code): $err"

d="$(newrepo renamed)"
(cd "$d" && printf 'check:\n\texit 1\n' >Makefile && git commit -qam "failing check" && git mv ok_test.go notes.txt)
stop_hook "$d"
[ "$code" = 2 ] && pass "Stop hook runs the checks when a Go file is renamed away" || fail "Stop hook ignored a Go file renamed to notes.txt ($code)"

d="$(newrepo fixture)"
(cd "$d" && printf 'check:\n\texit 1\n' >Makefile && git commit -qam "failing check")
mkdir -p "$d/internal/x/testdata" && printf 'data\n' >"$d/internal/x/testdata/case.txt"
stop_hook "$d"
[ "$code" = 2 ] && pass "Stop hook runs the checks when only a test fixture changed" || fail "Stop hook ignored a test fixture ($code)"

d="$(newrepo question)"
add_failing_test "$d"
printf 'Does M21.1 apply to "junctions" on Windows?\n\tTab, backslash \\ and a bell \a.\n' >"$d/QUESTION.md"
stop_hook "$d"
if [ "$code" = 0 ] && printf '%s' "$out" | valid_json && printf '%s' "$out" | grep -q 'junctions'; then
	pass "Stop hook allows a stop when QUESTION.md exists, and shows the question as valid JSON"
else
	fail "Stop hook with QUESTION.md: exit $code, output: $out"
fi

# --- SessionStart hook ---

session_hook "$d"
renamed="$(cd "$d" && ls QUESTION.unanswered-*.md 2>/dev/null)"
if [ "$code" = 0 ] && [ ! -e "$d/QUESTION.md" ] && [ -n "$renamed" ] && printf '%s' "$out" | valid_json; then
	pass "SessionStart renames a QUESTION.md left from an earlier session"
else
	fail "SessionStart: exit $code, QUESTION.md still there or not renamed: $out $err"
fi
stop_hook "$d"
[ "$code" = 2 ] && pass "a renamed question no longer lets a failing session stop" || fail "failing session stopped after the rename ($code)"

printf 'second\n' >"$d/QUESTION.md"
session_hook "$d"
count="$(cd "$d" && ls QUESTION.unanswered-*.md | wc -l | tr -d ' ')"
[ "$count" = 2 ] && pass "SessionStart never overwrites an earlier renamed question" || fail "expected 2 renamed questions, found $count"

d="$(newrepo noquestion)"
session_hook "$d"
[ "$code" = 0 ] && [ -z "$out" ] && pass "SessionStart does nothing without a question" || fail "SessionStart without a question: $code $out"

# --- review-diff.sh ---

d="$(newrepo review)"
(
	cd "$d" || exit 1
	printf 'review/\n' >.gitignore
	git add .gitignore && git commit -qm ignore
	start="$(git rev-parse HEAD)"
	printf 'changed\n' >>README.md
	git add README.md # something already staged, which must stay staged
	mkdir -p internal/newpkg
	printf 'package newpkg\n' >internal/newpkg/newpkg.go
	printf 'unstaged\n' >>go.mod
	before_index="$(git ls-files --stage | git hash-object --stdin)"
	before_cached="$(git diff --cached --name-only)"
	bash scripts/review-diff.sh "$start" >/dev/null
	after_index="$(git ls-files --stage | git hash-object --stdin)"
	after_cached="$(git diff --cached --name-only)"
	diff=review/milestone.diff
	ok=1
	grep -q '^+++ b/internal/newpkg/newpkg.go' "$diff" || { echo "  new package missing from diff"; ok=0; }
	grep -q '^+++ b/README.md' "$diff" || { echo "  staged change missing"; ok=0; }
	grep -q '^+unstaged' "$diff" || { echo "  unstaged change missing"; ok=0; }
	grep -q 'review/milestone.diff' "$diff" && { echo "  diff includes itself"; ok=0; }
	[ "$before_index" = "$after_index" ] || { echo "  index entries changed"; ok=0; }
	[ "$before_cached" = "$after_cached" ] || { echo "  staged set changed"; ok=0; }
	git ls-files --error-unmatch internal/newpkg/newpkg.go >/dev/null 2>&1 && { echo "  new file was added to the real index"; ok=0; }
	[ "$ok" = 1 ]
) && pass "review diff includes a new package and leaves git's index unchanged" || fail "review-diff.sh"

(cd "$d" && bash scripts/review-diff.sh not-a-commit >/dev/null 2>&1) && fail "review-diff.sh accepted a bad commit" || pass "review-diff.sh refuses a bad start commit"

echo
if [ "$failures" -gt 0 ]; then
	echo "test-scripts: $failures failure(s)"
	exit 1
fi
echo "test-scripts: all passed"
