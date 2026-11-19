# Spike prototype (throwaway)

The prototype for the spike in IMPLEMENTATION_PLAN.md: a minimal agent loop against the Claude API with Hermit's version 1 tools, to find out whether an agent this constrained is still worth using. It lives only on the `spike` branch and never merges into `main`. None of its code is reused.

Run it only on the spike's repositories, under the conditions in IMPLEMENTATION_PLAN.md. How sessions run and how results are judged is in [PROTOCOL.md](PROTOCOL.md).

## Build

    cd spike && go build -o ../bin/spike .

## Run

    HERMIT_SPIKE_ANTHROPIC_KEY="$(security find-generic-password -s hermit-spike-anthropic -w)" \
      bin/spike -repo ~/src/project -format replace -task 1 -who alice -task-file task1.md

- `-format`: the edit format under test (open decision 9): `replace` (exact-text replacement), `diff` (unified diff) or `whole` (whole-file writes).
- `-task`, `-who`: match the session to the recording sheet.
- `-task-file`: the task text, sent exactly as written as the first message.
- `-effort` (default `high`), `-model` (default `claude-opus-5-5`), `-reports` (extra report globs), `-log-dir` (default `~/hermit-spike-logs`).

At the prompt: type a message; `"""` on its own line starts and ends a multi-line message; `/stats` shows the measures so far; `/done` ends the task. Ctrl-C stops the agent's current turn.

## What it approximates

- **Tools (M1):** list, read, search, read test results, delete, and one edit tool per format. No shell, network or history.
- **Protected files (M8.1):** readable, never writable: build and CI configuration, manifests and lockfiles, tool and IDE settings, git internals, AI tool instructions, `.gitignore`.
- **Unavailable files (M13):** `.env*`, keys and credential files, git-ignored files and binary files are neither readable nor writable, and never appear in listings or search. Refusals read "not available", the same as a missing file.
- **Stale edits (M3.2):** an edit to a file the agent hasn't read, or that changed since it read it, is refused.
- **Reviewable writes (M6):** binary content and lines over 500 characters in code are refused.
- **Test results (M15):** JUnit XML from the usual report folders, with failures and stack traces, marked stale when the agent has edited since the run.

Not approximated: scanning (M19), safe rendering beyond stripping control characters (M20), plans and phases (M4), snapshots and undo, source control (M24), and the policy.

## The log

Each session writes a JSONL log: the developer's messages, every model turn (time, tokens, stop reason, text), every tool call (outcome, refusal kind, edit failures), and a closing summary. The summary has the measures the spike records automatically: model turns, time, edits attempted and failed for the format, and refusals by kind. The developer records the rest on the sheet: completion, test runs, whether they can explain the diff, and their rating.

The requests use Claude Opus 5.5 with adaptive thinking, prompt caching, and the API's server-side refusal fallback. A turn served by a fallback model shows up under `models` in the summary.
