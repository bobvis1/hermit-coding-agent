# Commands
- All checks: make check
- One package: go test ./internal/fsguard/...
- One test: go test ./internal/fsguard/ -run TestM21_1_SymlinkToSSH
- Fuzz one target for 60s: go test ./internal/fsguard/ -fuzz FuzzNormalize -fuzztime 60s
- Review diff for a milestone: scripts/review-diff.sh <start-commit>

# Decisions not in the code
- REQUIREMENTS.md is the spec. Cite requirement IDs in commit messages and test names, down to the numbered part where there is one (M21.1 becomes M21_1 in a test name).
- Never change REQUIREMENTS.md to match the code. Stop and ask instead; the project maintainer approves every change to it.
- Repository files only through internal/fsguard; local storage only through internal/localstore.
- If a requirement is ambiguous or a test can't pass without changing the spec, write the question to QUESTION.md and stop.
