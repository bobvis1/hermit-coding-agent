# Implementation Plan: Hermit

Status: draft. The specification is [REQUIREMENTS.md](REQUIREMENTS.md); requirement IDs (M1–M24, S1–S7, P1–P12) refer to it. Milestones are numbered 0–14 and are never written with an "M", to keep them distinct from requirement IDs. Three pieces of work are named rather than numbered: the spike before milestone 1, the walking skeleton after milestone 5, and Bitbucket support after milestone 10.

This plan assumes Hermit is built with an AI coding agent (Claude Code), following Anthropic's [best practices for Claude Code](https://code.claude.com/docs/en/best-practices). The practices that shape it:

- **Build from a written spec, in fresh sessions.** REQUIREMENTS.md is the definition of done. Each milestone starts in a clean session that reads the relevant requirements, rather than carrying a long conversation forward.
- **Give the agent a check to run.** Every milestone ends in tests that pass or fail. The checks exist before the code they check, especially for security rules.
- **Enforce what can't have exceptions; write down only what can't be found in the code.** The repository's own `CLAUDE.md` stays short. Rules that must always hold become hooks, deny rules or CI checks.
- **Review against the spec with a reviewer that didn't write the code.** A read-only reviewer compares each milestone's diff with the requirements it claims to implement, and reports only correctness gaps.
- **A person has the last word.** A developer reviews every milestone's diff and makes the commit, the same rule Hermit enforces for its own users (principle 2).

## 1. Architecture

One Go module that builds the `hermit` executable (M18). Packages are split so that the security-critical parts are small, separately tested, and have no dependency on the model or the network.

| Package | Responsibility | Requirements |
|---|---|---|
| `cmd/hermit` | Entry point, subcommands (`hermit`, `hermit doctor`, `hermit undo`) | M12, S3, M3 |
| `internal/policy` | Load, validate and recheck the organization policy; managed-device detection; data class resolution | M14, M18, M23 |
| `internal/credstore` | Credential storage: the operating system's credential store, the kernel keyring on headless Linux, or sign in each session | M18 |
| `internal/localstore` | Encrypted local storage for everything the harness keeps on the developer's machine (snapshots, pending changes, conversation state, the local log), with its key in `credstore`; retention and cleanup | M10.11–M10.15 |
| `internal/fsguard` | The only code that touches the repository's files: root confinement, path normalization, protected, unreadable and allowlisted paths, file-type detection, atomic and conditional writes | M8, M13, M21, M6 |
| `internal/scan` | Secrets, organization patterns, personal data, health pack, reserved test values | M19 |
| `internal/render` | Escape stripping, hidden and bidirectional characters, link and image handling, on-screen masking | M20 |
| `internal/tooldef` | The `Set` type carrying tool names, descriptions and schemas to `provider`; only `tools` may construct one | M1 |
| `internal/tools` | The agent's fixed tool set, each tool a thin layer over `fsguard`, `scan`, `scm` and `testresults`; tracks what the agent has read so stale edits are refused | M1, M2, M3, M9 |
| `internal/loop` | The agent loop: messages, tool calls, condensing long conversations, standing instructions | M1, M11, S7 |
| `internal/provider` | Provider interface and adapters: Anthropic, Bedrock, OpenAI-compatible, gateway, scripted | M17, S4 |
| `internal/worktree` | Snapshots, undo, phase file lists, pending-changes mode | M3, M4, M22 |
| `internal/testresults` | JUnit XML parsing (reading reports through `fsguard`), freshness, detail levels | M15 |
| `internal/gitcmd` | The only package that runs a process: a fixed list of git subcommands, used for the remote, the worktree, the git-ignored check and undo | M2, M3, M13, M23 |
| `internal/scm` | Fixed source control calls for GitLab and GitHub | M24, S2 |
| `internal/audit` | Log tiers, keyed hashes, hash chain, upload, anchoring, on top of `localstore` | M10.1–M10.10, M16 |
| `internal/netclient` | The one HTTP client for every outbound connection: proxy, extra CA certificates, HTTPS except on loopback | M14, M17 |
| `cmd/hermit-audit` | Organization-run tooling, not on developer machines: log cross-checks, anchor checks, alerts, investigator search | M16 |
| `internal/tui` | Terminal interface on the alternate screen | M12, M20, S1 |

**Required paths.** Five rules carry most of the security, so each is enforced by the compiler or by CI, not by convention:

| Rule | How it is enforced |
|---|---|
| Every repository read and write goes through `fsguard`, including test reports | A static check can see a file call but not which path it gets, so the CI check is by package: only `fsguard`, `localstore` and `policy` (the policy file, the CA certificate files it names, and managed-device markers) may use file-system APIs (the file functions in `os`, `io/fs` walking, and file calls in `syscall` and `golang.org/x/sys`). Of these, only `fsguard` is given repository paths, which review checks. `testresults` reads report files through `fsguard`, so M13 applies to them |
| Every byte sent to the model passes through `scan` | Compiler: the provider interface accepts only `scan.Cleared`, a type with unexported fields that only `scan` can construct. Text the harness itself writes (system prompt, standing instructions) uses `scan.Fixed`, whose parameter has an unexported string type, so callers outside `scan` can pass only string constants, never variables. Tool definitions are structured data (names, descriptions, JSON schemas generated from Go types), so they reach the provider as a `tooldef.Set` from a small package `internal/tooldef`, which `provider` may import; CI allows only `tools` to call its constructor. CI: `provider` may not import `fsguard`, `tools`, `scm` or `testresults`, so it has no other source of content |
| Every byte displayed, and every MR text sent, passes through `render` | Compiler: `tui` accepts only `render.Safe`, and `scm` accepts MR text only as `render.ApprovedMRText`. CI: only `tui`'s approval step may call the constructor for `ApprovedMRText` (M20), because the compiler alone can't restrict which package calls an exported function |
| Every tool call is logged | Tools run only through one wrapper in `loop` that records the call before and after. A test enumerates every tool and checks that calling it produces audit events |
| Only `git` runs, and only through one package | CI check: only `internal/gitcmd` may start a process, whether through `os/exec`, `os.StartProcess`, `syscall.Exec`, `syscall.ForkExec` or their `golang.org/x/sys` equivalents. It runs a fixed list of git subcommands. The check is an analyzer run over every package linked into `hermit`, third-party ones included, so a dependency that starts programs fails it too. That rules out the AWS SDK's `config` package, whose default loading runs any `credential_process` in `~/.aws/config`: AWS clients get their credentials from Hermit's own IAM Identity Center sign-in. It also rules out credential store libraries that run `/usr/bin/security` on macOS (open decision 8). Sign-in flows show the URL for the developer to open, rather than starting a browser. Everything that needs git (policy reading the remote, the worktree, the git-ignored check, undo) goes through `gitcmd`. This is what makes "the agent runs no commands" (M2) checkable, and lets a test enumerate every git invocation. `gitcmd` also stops git from starting other programs: every call passes `--no-pager` and `-c core.fsmonitor=false`, any diff compares stored objects only and adds `--no-ext-diff --no-textconv`, and subcommands that would run clean or smudge filters on working-tree content are left off the list. That includes `git status` and any diff against the working tree, which run clean filters on changed files whatever `--no-textconv` says; the harness works out changes from its own snapshots instead, and computes blob hashes for read records in Go. Network subcommands such as `fetch`, `pull` and `ls-remote` are never on the list. A hostile test uses a repository whose configuration sets an fsmonitor program, clean and smudge filters, a textconv driver, an external diff tool and a pager, and checks that none of them runs |

The dependency checks apply to non-test code only. Test files and test fixtures may do what the rules forbid, for example creating hard links or building a fixture that should fail to compile.

Other dependency rules, also checked in CI from milestone 0:
- `fsguard`, `scan` and `render` import nothing from the rest of Hermit.
- Everything stored on the developer's machine goes through `localstore`.
- Every outbound network connection goes through `netclient`; no other package imports `net/http` directly.

## 2. How each milestone is run

1. **Start a fresh session** for the milestone. Name it after the milestone so it can be resumed.
2. **Point at the spec.** For example: *"Implement milestone 3 from IMPLEMENTATION_PLAN.md. The requirements are M21, M8, M13 and M6 in REQUIREMENTS.md. Write the failing tests for every case listed in M21 and S4 first, then implement until `make check` passes."*
3. **Tests first for security rules.** For `policy`, `localstore`, `fsguard`, `scan` and `render`, the hostile cases from the requirements are written as failing tests before the implementation.
4. **Run the checks.** The Stop hook (milestone 0) runs `make check` whenever the agent tries to finish with Go changes, so a milestone can't end with failing tests. Ask for the command output as evidence, not a summary. If the agent needs to stop and ask instead (below), it writes the question to `QUESTION.md`, which lets it stop.
5. **Review against the spec.** The main session saves the milestone's full diff, including new files, against the commit where the milestone started: `scripts/review-diff.sh <start-commit>` writes `review/milestone.diff` (`review/` is gitignored). Plain `git diff` would leave out the new packages most milestones create, so the script stages everything into a temporary copy of git's index (`GIT_INDEX_FILE`) and diffs that, leaving the developer's own index untouched. It then asks the read-only `spec-reviewer` subagent to compare that file with the milestone's requirements: every requirement implemented, every listed case tested, gaps that affect correctness only, no style comments. The reviewer reads the diff from the file, so it needs no shell.
6. **A developer reviews and commits.** Nothing merges without a person reading the diff.
7. **Clear between milestones.** Unrelated history makes later sessions slower and more expensive.

**Effort.** Use high effort for milestones 1, 2, 3, 4, 5, 7, 8 and 9, where a missed edge case is a security bug: fail-closed policy loading, local storage and credentials, the file access guard, scanning and rendering, the agent loop that wires every compile-time guarantee together, which provider sees which repository's data, logging, and the source control limits. Default effort is enough elsewhere.

**Parallel work.** Some milestones are independent (section 4). Run them as separate sessions in separate worktrees so their edits can't collide, and merge them one at a time.

**When to stop and ask.** If the requirements turn out to be wrong, ambiguous or contradictory, the session writes its question to `QUESTION.md` and stops (the Stop hook allows this, milestone 0). Changes to REQUIREMENTS.md are made by a person, never as a side effect of implementing a milestone.

## 3. Milestones

Each milestone lists what it delivers and its checks: what must pass before it counts as done.

### Milestone 0: Repository and checks

Delivers:
- Go module pinned to Go 1.25 or later, in `go.mod` and in CI. `fsguard` needs `os.Root.Rename` for atomic writes inside the root, and the hostile tests need `os.Root.Link` to create hard links; both arrived in Go 1.25. (Go's FIPS mode, mentioned in M18, has been available since 1.24, so 1.25 covers both.)
- Package layout from section 1, `Makefile` with `make check` (format check, `go vet`, `staticcheck`, unit tests, the dependency-rule checks, including the process check over every linked package) and `make fuzz`.
- `scripts/review-diff.sh` for the review step in section 2.
- CI running `make check` on macOS, Windows and Linux (S4 requires all three).
- Development guardrails for building Hermit with Claude Code:
  - a short `CLAUDE.md` (draft below);
  - `.claude/settings.json` with the `Stop` hook below; `ask` for edits to `REQUIREMENTS.md`; `deny` for reading credential files;
  - `.claude/agents/spec-reviewer.md`: tools limited to `Read, Grep, Glob`, instructed to compare the diff file it is given against named requirements and skip style;
  - `.gitignore` entries for `review/` and `QUESTION.md`;
  - `.claude/rules/security-core.md` with `paths:` set to the security packages, holding the conventions for that code (below).

**The Stop hook** (`scripts/stop-check.sh`) blocks the agent from finishing while `make check` fails, with two exceptions:
- **No Go changes:** if no `.go` files, `go.mod`, `go.sum`, test fixtures or the `Makefile` changed in the working tree, it allows the stop, so documentation-only sessions aren't held up.
- **A question is pending:** if `QUESTION.md` exists, it allows the stop and shows the question to the developer. This is how "stop and ask" (section 2) works when the requirements are ambiguous. The developer answers, deletes the file and resumes the session.

A `SessionStart` hook, limited to the `startup|resume` matcher so it doesn't fire when a session is compacted or cleared partway through, keeps the question file from turning the Stop hook off for good. At the start of a session it renames any existing `QUESTION.md` to `QUESTION.unanswered-<date>.md` and warns the developer. So a `QUESTION.md` that the Stop hook sees was always written in the current session.

The Stop hook does not use `stop_hook_active` as an exception, because that would let the agent stop after a single retry with tests still failing. An endless loop is prevented by Claude Code itself: per the [best-practices page](https://code.claude.com/docs/en/best-practices#enforce-the-check-with-a-stop-hook), after eight blocked stops in a row with no tool call between them, it overrides the next block and ends the turn (`CLAUDE_CODE_STOP_HOOK_BLOCK_CAP` changes the number). Using the question file to get past failing tests is visible to the developer, who sees the question.

Checks: `make check` passes on all three operating systems in CI; the Stop hook blocks a deliberately failing test, allows a documentation-only change, and allows a stop when `QUESTION.md` exists; a `QUESTION.md` left from an earlier session is renamed at session start and no longer lets a failing session stop; the review diff includes a newly created package and leaves git's index unchanged; each dependency rule fails on a deliberate violation, including a test dependency that starts a process.

Draft `CLAUDE.md`:

```markdown
# Commands
- All checks: make check
- One package: go test ./internal/fsguard/...
- One test: go test ./internal/fsguard/ -run TestM21_SymlinkToSSH
- Fuzz one target for 60s: go test ./internal/fsguard/ -fuzz FuzzNormalize -fuzztime 60s

# Decisions not in the code
- REQUIREMENTS.md is the spec. Cite requirement IDs in commit messages and test names.
- Never change REQUIREMENTS.md to match the code. Stop and ask instead.
- Repository files only through internal/fsguard; local storage only through internal/localstore.
- If a requirement is ambiguous or a test can't pass without changing the spec, write the question to QUESTION.md and stop.
```

Draft `.claude/rules/security-core.md`:

```markdown
---
paths:
  - "internal/policy/**"
  - "internal/credstore/**"
  - "internal/localstore/**"
  - "internal/netclient/**"
  - "internal/fsguard/**"
  - "internal/scan/**"
  - "internal/render/**"
---
- Fail closed: any error or unknown case refuses, never allows.
- Every rule needs a test for the allowed case and the refused case.
- Name hostile tests after the requirement and case, for example TestM21_SymlinkToSSH.
- No dependencies outside the standard library without asking.
```

### Spike: is the constrained agent useful?

Requirements: M11, and the constraints that most limit usefulness: M1, M2, M7, M8, M13. Runs alongside milestone 0, before milestone 1.

Goal 2 makes usefulness a security requirement, and nothing else in the plan tests it before a real model is wired in at milestone 7. The spike is a throwaway prototype, kept out of the main branch, that answers one question early: is an agent with no shell, no test runs, no history and no access to manifests or ignored files still worth using?

Delivers: a minimal loop against the Claude API with the version 1 tool set (read, search, list, edit, read test results), with the protected and unreadable paths approximated. Two or three developers use it for a few days on 5–10 real tasks in repositories with no sensitive data, and a short findings note records:
- how many tasks finished, and how many developer test runs each took;
- how often protected manifests, ignored files (including generated code) or missing history got in the way;
- how often edits failed to apply, and which edit format worked best (open decision 9).

Checks: the findings note exists and has been read by whoever owns REQUIREMENTS.md. Findings that would change the requirements go to `QUESTION.md`, as for any milestone. None of the prototype's code is reused.

### Milestone 1: Policy

Requirements: M14 (all parts; M14.4–M14.9 as the schema), M18 (policy location, refuse writable policy, fail closed, managed-device detection, recheck before each request, unmanaged mode, minimum harness version), M23 (class ranking and repository matching). High effort. Can run in parallel with milestones 3 and 4.

Delivers: `internal/policy` with schema validation of every setting in M14.4 to M14.9, per-OS policy locations, the minimum-version check, and data class resolution from the git remote; and `internal/gitcmd` with its first fixed subcommand (reading the remote). Later milestones add subcommands to that fixed list, and each addition shows up in review.

Checks:
- A harness older than the policy's minimum version refuses to start.
- `gitcmd` hostile test: a repository configured with an fsmonitor program, filters, a textconv driver, an external diff tool and a pager runs none of them.
- A malformed, unreadable or developer-writable policy stops the harness, and so does one missing a required setting (M14.3). The schema has no field that can hold a secret, such as an API key (M14.2). So does a policy in a folder the developer can write to, or one reached through a symbolic link. On Windows, rights the developer has through a group or an inherited permission count.
- A missing policy on a device with a managed marker stops the harness; with no marker, unmanaged mode.
- An unmatched repository gets the most restrictive ranked class. A repository with several remotes gets the most restrictive class among them, and URL rewriting in git configuration is applied before matching.
- A changed policy file is picked up by the recheck; `enabled: false` stops the next request.
- The example policy in REQUIREMENTS.md validates.

### Milestone 2: Credentials, local storage and network client

Requirements: M18 (credential storage and fallbacks), M10.11–M10.15 (files on the developer's machine, session maximum age, `hermit purge`), M14.9 (HTTPS proxy and extra CA certificates), M17 (HTTPS except on loopback). High effort. Can run in parallel with milestones 3 and 4.

Delivers: `internal/credstore` (operating system credential store, kernel keyring on headless Linux, sign in each session; on macOS, reaching the Keychain without cgo or running a program, per open decision 8); `internal/localstore` (encryption at rest with its key in `credstore`, readable only by the developer's account, retention, session maximum age and cleanup rules from M10, removal on uninstall); and `internal/netclient`, the one HTTP client every outbound connection uses (model providers, source control, log upload), applying the proxy, extra CA certificates and the HTTPS-only rule.

Checks:
- `netclient` refuses plain HTTP except to loopback addresses, and uses the configured proxy and CA certificates (tests against local test servers with a private CA).
- Nothing written through `localstore` is readable as plaintext on disk (test reads the raw files).
- Files are created with owner-only permissions on each operating system.
- Retention and cleanup: session files removed at session end or maximum age; local logs removed only after confirmed upload plus the retention period.
- Each `credstore` backend round-trips a secret, and "sign in each session" stores nothing.
- `hermit purge` removes every local file and credential store entry for the user; files left without their key are unreadable.

### Milestone 3: File access guard

Requirements: M21, M8, M13, M6. High effort. Can run in parallel with milestones 1, 2 and 4: `fsguard` imports nothing else from Hermit, and gets git-ignored answers through an interface (open decision 6), so it doesn't wait for `gitcmd`.

Delivers: `internal/fsguard`, the single gate for repository file access: root-confined operations (`os.Root`), resolved-path rule checks, case- and Unicode-insensitive matching, Windows name rules, multiply-linked file refusal, `.git` in every form, atomic writes that keep permissions, refusal of writes to symbolic links, a conditional write that succeeds only if the file still matches a given content hash (used for stale-edit detection in milestone 5), protected and unreadable lists loaded from per-ecosystem data files, `.agentignore` (unreadable and unwritable), read and write allowlists, git-ignored files (neither readable nor writable), binary and container detection by content, notebook output stripping, and refusal of hard-to-review writes.

Checks:
- Every M21 case from S4's list, as tests on all three operating systems.
- A conditional write fails if the file changed after the given hash was taken.
- Refused reads return the same "not available" result whether or not the file exists, and unreadable names never appear in listings or search.
- Writes to git-ignored paths (`.env`, a `.pth` file under `.venv/`, a file under an organization-readable `node_modules/`) and to unreadable paths are refused, with the same "not available" result for unreadable ones.
- A write to a tracked symbolic link is refused and leaves the link in place; an atomic write keeps the executable bit; on Windows, a write to a file another program holds open is refused and leaves no partial file.
- A long Markdown paragraph is accepted; a long line of code is refused.
- Fuzz targets for path normalization and matching run in CI.

### Milestone 4: Scanning and rendering

Requirements: M19, M20. High effort. Can run in parallel with milestones 1, 2 and 3: `scan` and `render` import nothing else from Hermit.

Delivers: `internal/scan` (secrets, organization patterns, opt-in personal data and health pack, reserved test values, redact and block modes) and `internal/render` (escape stripping, hidden and bidirectional characters, link and image handling for MR text and markdown, masking).

Checks:
- Table-driven tests from fixture files: known token formats, HL7v2, FHIR, DICOM marker, check-digit identifiers, reserved test ranges and test-value secrets (`...EXAMPLE`, repeated characters) not flagged.
- Escape-sequence and Unicode fixtures, including Trojan Source examples and OSC 8 links.
- Fuzz targets for the scanner and the renderer.

### Milestone 5: Agent loop and scripted provider

Requirements: M1, M2, M9, M3 (stale-edit detection, edits to altered content), M10.1 (through the audit event interface), M10.14 (resume rules, condensed summaries as transcripts), M10.16 (one repository per session), M19 and M20 (scanning and checks on writes), S4, S5 (tool-call parser), S7. Depends on milestones 2, 3 and 4. High effort. This is the largest milestone, so run it as two sessions: first the tools and their wiring to `fsguard` and `scan`, then the loop, condensing and resume.

Delivers: the tool design: how an edit names what it changes, how edits apply to the real file when the model's view was altered (M3), and the shape and result sizes of search, listing and reads. It starts from the spike's findings and open decision 9, and is measured with the first evaluation tasks (milestone 13). Then `internal/tools` (the fixed tool set, no shell or network tools; read tracking and stale-edit refusal using `fsguard`'s conditional write; edits that overlap redacted or removed text refused; scanning and hidden-character checks on what each write adds), `internal/loop` (with every model-bound message passing through `scan` and every displayed message through `render`, including the model's own earlier turns and condensed summaries, which are scanned again before being resent rather than given a way around `scan`; one repository per session; conversation state and condensed summaries kept in `localstore` as local transcripts; resume only within the session maximum age), the provider interface, including a hook for per-request metadata (milestones 7 and 8), the audit event interface with an in-memory recorder for tests (milestone 8 implements the real one), `scan.Fixed` for harness-written text, `internal/tooldef` for tool definitions, and the scripted provider. Standing instructions (S7).

Checks:
- Scripted hostile sessions from S4 (write `.git/hooks/pre-commit`, read `.env`, overwrite a newer developer edit, malformed tool calls) are all refused, with the refusal recorded.
- A planted secret in a file the agent reads reaches the scripted provider redacted.
- An edit overlapping the redacted secret is refused, and no redaction marker is ever written to disk; an edit elsewhere in that file succeeds. A write that adds a secret is refused.
- Editing a notebook cell clears that cell's outputs and keeps the other cells' outputs.
- A Markdown write that adds an image from a domain that isn't allowed is refused; a write that adds bidirectional or tag characters to code is refused.
- Code that passes a string variable to `scan.Fixed` doesn't compile; a string constant does.
- The required paths in section 1 hold: code that passes a plain string to a provider doesn't compile, a `tooldef.Set` built outside `tools` fails the CI check, and every tool in the enumerated tool set produces audit events in the in-memory recorder when called.
- No tool exists that runs a command or opens a network connection: a test enumerates the tool set.
- The scripted provider can't be enabled through organization policy.
- A session can't switch repositories, and resuming after the session maximum age is refused.
- A fuzz target for the tool-call parser runs in CI (S5).

### Walking skeleton: first usable build

Requirements: M11, M12 (minimal), M17 (the Claude API adapter only). Depends on milestone 5. Can run alongside milestones 6–9.

Delivers: the smallest Hermit a developer can use: the Claude API adapter, using `credstore` and `netclient`, and a minimal terminal interface on the alternate screen that shows only `render.Safe` text. The team uses it on Hermit's own repository and on non-sensitive repositories, with the first evaluation tasks run against it. Milestones 7 and 11 build on this adapter and interface rather than starting again.

Checks: a developer completes a small real task from plan to reviewed diff; the evaluation tasks written so far run against it end to end.

### Milestone 6: Working tree

Requirements: M3 (snapshots and undo), M4, M5, M8 (flagging edits to scripts CI calls), M22.

Delivers: `internal/worktree`: snapshots in `localstore`, `hermit undo` including untracked files, phase file lists and the unplanned-edit report, sensitive-path notes, phase size measured from snapshots as M4 defines it, with warnings at plan approval, in the next tool result when a phase first crosses the threshold, and in the phase summary, plus sizes recorded in audit events, pending-changes mode, and flags in the change summary for edits to scripts that CI configuration references (found by reading known CI formats through `fsguard`).

Checks: scripted sessions that edit, delete and create files, then undo; undo of a step whose file the developer changed afterwards reports the conflict and leaves the developer's version unless they choose otherwise; the harness never runs `git commit` or `git push` (a test enumerates git invocations); an edit to a script referenced from GitLab CI and GitHub Actions fixtures is flagged, not refused. Phase size:
- table-driven tests of the measure: a pure rename counts as no lines; a deleted file counts as one file and no lines; whitespace-only changes don't count; lines in allowlisted hard-to-review paths are reported separately; the developer's own uncommitted changes don't count;
- a plan whose phase lists more files than the threshold is flagged at approval;
- a scripted session that crosses the line threshold gets exactly one note, in the next tool result, and the work is never refused;
- the default threshold applies when the policy doesn't set one, and a policy value overrides it.

### Milestone 7: Model providers and data classes

Requirements: M17, M23 (provider and feature gating, local-listener verification, repository identity and class sent to the gateway), M14.6 (usage limits). High effort. Can run in parallel with milestones 6, 8 and 9.

Delivers: adapters for Bedrock, OpenAI-compatible endpoints and the organization gateway, alongside the walking skeleton's Claude API adapter, using `credstore` for keys and tokens and `netclient` for connections; approved-model and feature enforcement per class; listening-process check for local-only classes, made on the connection the harness opened and identifying the program by executable path and signature; usage limits per session and per day, counted from provider responses and kept in `localstore` (these counters are advisory, because a developer can delete local files to reset them; the gateway is the real enforcement); per-request metadata sent to the gateway. This milestone owns the metadata mechanism and the repository identity and class it carries; milestone 8 adds the log chain head to it.

Checks:
- Adapter contract tests against recorded responses, no live calls in `make check`.
- An unapproved model, cross-region profile or feature is refused before any request.
- A loopback port held by `ssh` or an unidentified process is refused for a local-only class (on macOS, subject to open decision 5). So is a process named `ollama` running from an unapproved path, and a model ID not on the class's list, such as an Ollama cloud model.
- Requests stop with a clear message once the session or daily usage limit is reached.
- Gateway requests carry the repository identity and class (recorded-request test).
- Every AWS SDK client (Bedrock, and in milestone 8 S3 and KMS) and any other SDK client is built with `netclient`'s transport, never the SDK's default HTTP client, so the proxy, CA and HTTPS rules apply. A test inspects each constructed client.
- A manual smoke test against a local model (Ollama), as described in S4.

### Milestone 8: Logging

Requirements: M10.1–M10.10, M10.12 (deletion after confirmed upload), M16. High effort. Can run in parallel with milestones 6, 7 and 9.

Delivers:
- `internal/audit` on top of `localstore`, implementing the audit event interface from milestone 5: three tiers, keyed hashes with a per-session key, wrapped locally with the organization's public wrapping key from the policy (no credentials, network or KMS call needed, for every log destination), read records (commit, blob hash and line range, only for content that matches `HEAD`), escrow option, per-session data keys for the erasable tiers, kept in the session key store (M16), condensed summaries logged as transcripts, hash chain, anchoring (the chain head added to each gateway request through milestone 7's metadata mechanism, or uploaded to S3 every minute without a gateway), batched S3 upload with retry.
- `cmd/hermit-audit`, separate tooling the organization runs on its own infrastructure, not on developer machines: scheduled cross-checks of Hermit's logs against the gateway's request log, CloudTrail or provider usage records, checks that uploaded logs reach their anchors, alerts on gaps, and investigator search by keyed hash, unwrapping each session's hashing key through KMS.
- AWS setup templates for the bucket, session key store, keys and roles in M16 (for P5), with write-once uploads enforced in the bucket policies, and for running the cross-checks; the asymmetric wrapping key, with decryption limited to an investigator role; and a documented, scripted procedure for each wrapping-key period: create the new key, publish its public key in the policy, and schedule the old key's deletion.

Checks:
- Cross-checks flag model usage with no matching session, and a session whose uploaded log stops short of its anchor (tests against fixture records).
- No plaintext path, query or argument appears in an audit event (property test over generated sessions).
- A read of a tracked file with uncommitted changes is recorded as content that can't be rebuilt, not with a blob hash.
- The upload role can't overwrite a log chunk, an anchor or a session key it has written, or delete a session key (tested against the AWS templates).
- Hashing and wrapping work with no network and no credentials, for each log destination; a wrapped session key can't be unwrapped with any developer or upload role, only the investigator role (tested against the AWS templates).
- After a policy update with a new public key, new sessions wrap with the new key, and earlier sessions keep the key ID they were wrapped with.
- Truncating the local log is detected against the anchor.
- Upload tests against a local S3-compatible server; a separate manual test against a real AWS account, verifying Object Lock modes and crypto-shredding.

### Milestone 9: Source control

Requirements: M24, S2, M20 (MR text approval and traceability), M23 (checking the repository's claimed identity with the host). GitLab first, then GitHub; Bitbucket is separate work after milestone 10. High effort. Can run in parallel with milestones 6, 7 and 8.

Delivers: `internal/scm` with the fixed call list, current-project check, the commit check for repository identity (M23), branch rules, diff filtering by read rules, host content passed through `scan` and `render`, exact-text approval with session ID and phases, and retrieval of CI test reports.

Checks: tests against recorded API responses; a test enumerates the client's endpoints and fails if one outside the fixed list appears; MR diffs touching unreadable files are filtered; MR text can be sent only as `render.ApprovedMRText`; a repository whose remote points at a project that shares no commit with it gets the most restrictive class.

### Milestone 10: Test results

Requirements: M15, M7. Depends on milestone 6 for snapshots (freshness) and milestone 9 for CI results.

Delivers: `internal/testresults`: JUnit XML parsing with size limits and no external entities, freshness against snapshots, detail levels per class, and summaries of CI test reports retrieved through `scm`.

Checks: fixtures from Maven, Gradle, pytest and Jest; malformed and oversized XML; a fuzz target.

### Bitbucket support

Requirements: M24, S2, M1 (CI results), M14.4 (host type and credential method), M15 (CI detail levels), M20 (PR text approval), M23 (commit check). Depends on milestones 9 and 10. Can run alongside milestones 11–13. High effort, as for milestone 9.

M14.4 names Bitbucket as a source control host, and many organizations use it, but it differs from GitLab and GitHub enough to be its own piece of work. It isn't on the critical path: GitLab and GitHub organizations don't wait for it.

Delivers: two `scm` adapters, one for each Bitbucket product, since they have separate APIs:
- **Bitbucket Cloud** (REST API 2.0);
- **Bitbucket Data Center**, self-hosted, with its own REST API.

Each adapter has the same fixed call list as GitLab and GitHub. The current project is the workspace and repository (Cloud) or the project and repository (Data Center). Behaviour:
- **Pull requests:** list and read pull requests and their comments; create or update a pull request's title and description only as `render.ApprovedMRText`; filter diffs by the read rules.
- **Identity:** the commit check for repository identity (M23).
- **Never:** merging, approving, declining, review tasks or comments (M24).
- **Credentials (open decision 10):**
  - **Cloud:** no documented device sign-in, and app passwords stopped working in June 2026, so developers use an API token with the narrowest scopes that read the repository and pull requests and write pull requests. The exact scope names come from the token creation screen.
  - **Data Center:** HTTP access tokens, repository-scoped where the organization allows.
  - **Policy:** neither puts a secret in the policy (M14.2).
- **CI results (M24, "CI detail depends on the host"):**
  - **Cloud:** the Pipelines API offers a test report summary per step: how many tests ran and how many failed. Per-test failure messages aren't available through the API (Atlassian request BCLOUD-24022). The agent gets the counts and the failed step, and relies on local reports (M15) for details.
  - **Data Center:** no built-in CI. It reads build statuses posted by external CI (Jenkins, Bamboo).
  - **Code Insights reports:** where the organization's CI publishes them, either adapter reads them as host content: scanned, hidden characters removed, untrusted (M19, M20, M24).

Checks:
- tests against recorded responses for both products;
- the endpoint enumeration test from milestone 9 covers both adapters;
- a pull request diff touching an unreadable file is filtered;
- pull request text can be sent only as `render.ApprovedMRText`;
- a repository whose remote points at a project that shares no commit with it gets the most restrictive class;
- with only test counts available, the agent's CI summary says so rather than inventing failure details;
- `hermit doctor` (S3) reports a token missing a required scope, by name.

### Milestone 11: Terminal interface and setup check

Requirements: M12, S1, S3, M20 (display).

Delivers: `internal/tui` on the alternate screen, grown from the walking skeleton's interface, plan approval, phase summaries, flagged commands in prose, paste handling per class, the startup class summary, support details from the policy shown at startup and in every error; `hermit doctor`.

Checks: rendering tests from recorded sessions; every error path shows the support contact; `hermit doctor` reports each failure with the setting to fix; a usability walkthrough in Eclipse's Terminal view and VS Code's integrated terminal.

### Milestone 12: Packaging and distribution

Requirements: M18.

Delivers: reproducible builds, signed and notarized macOS package, signed MSI, deb and rpm, SBOM and build provenance, FIPS build, MDM profile and ADMX templates, documented organization signing.

Checks: two independent builds produce identical executables; packages install and uninstall cleanly on each OS; uninstall, or the device-management script where the package format has no uninstall step (macOS), runs `hermit purge` for each user and leaves nothing readable (M10.15); developer packages contain `hermit` only, not `cmd/hermit-audit`, which ships separately for the organization's own infrastructure.

### Milestone 13: Evaluation suite

Requirements: M17 (evaluation before approval), M11. Depends on milestones 5 and 7, so it can run alongside 8–12. Organizations need it to approve a model before a pilot, so finishing it early shortens the path to a pilot. The first tasks are written during milestone 5 and run against the walking skeleton, so the tool design is measured from the start; this milestone completes the suite and adds providers.

Delivers: the evaluation suite organizations run before approving a model: scripted tasks on sample repositories that measure tool-call reliability, plan quality, phase size (using `worktree`'s measure, as M4 defines it, so evaluation and the pilot agree), how the model responds to the threshold note, and whether the model keeps within the rules, with a report per model.

Checks: the suite runs end to end against at least two providers, including a local model.

### Milestone 14: Validation and documentation

Requirements: S5, S6, M22 (IDE guidance), section 12 (operations), M11 (pilot measures).

Delivers: the validation support pack; recommended IDE settings; administrator documentation, including the kill switch and incident runbook; pilot materials for the success measures in section 9.

Checks: a reviewer who did not build Hermit can install and configure it from the documentation alone; the validation pack includes current results from S4, S5 and the evaluation suite.

## 4. Order and parallel work

| Milestone | Depends on | Can run alongside |
|---|---|---|
| 0 Repository and checks | — | Spike |
| Spike | — | 0 |
| 1 Policy | 0, spike | 3, 4 |
| 2 Credentials, local storage, network client | 1 | 3, 4 |
| 3 File access guard | 0 | 1, 2, 4 |
| 4 Scanning and rendering | 0 | 1, 2, 3 |
| 5 Agent loop | 1, 2, 3, 4 | — |
| Walking skeleton | 5 | 6, 7, 8, 9 |
| 6 Working tree | 5 | Skeleton, 7, 8, 9 |
| 7 Model providers and data classes | 5, skeleton | 6, 8, 9 |
| 8 Logging | 5 | Skeleton, 6, 7, 9 |
| 9 Source control | 5 | Skeleton, 6, 7, 8 |
| 10 Test results | 6, 9 | 7, 8 |
| Bitbucket support | 9, 10 | 11, 12, 13 |
| 11 Terminal interface and setup check | Skeleton, 6, 7, 8, 10 | Bitbucket |
| 12 Packaging and distribution | 11 | — |
| 13 Evaluation suite | 5, 7 | 8–12 |
| 14 Validation and documentation | 12, 13 | — |

- **Critical path:** policy, then credentials and local storage, then the agent loop, with the file access guard and scanning and rendering built alongside. The loop is where every model-bound message passes, so nothing in the product that talks to a model is built before `fsguard`, `scan`, `render` and `localstore` exist. The spike is the exception: it is thrown away, never merged, and runs only on repositories with no sensitive data.
- **Parallel tracks:** milestones 3 and 4 import nothing else from Hermit, so they can start straight after milestone 0 and run alongside 1 and 2. The walking skeleton and milestones 6, 7, 8 and 9 can run at the same time. Milestone 10 waits for 6 and 9: it checks freshness against snapshots and summarizes CI results retrieved through `scm`.
- **First usable build:** the walking skeleton, straight after milestone 5. Dogfooding from that point surfaces usability problems early (M11), while the spike has already answered whether the constrained agent is worth building at all.

## 5. Traceability

| Requirement | Milestone | Requirement | Milestone |
|---|---|---|---|
| M1 Capabilities | 5 | M13 Unreadable files | 3 |
| M2 No shell, network, credentials | 0 (process check), 2, 5 | M14 Organization policy | 1, 2, 7, 11 |
| M3 Shared working tree | 3, 5, 6 | M15 Test results | 10 |
| M4 Plan and phases | 6 | M16 Central log storage | 8 |
| M5 Developer commits | 6 | M17 Model providers | 2, skeleton, 7, 13 |
| M6 Reviewable writes | 3, 5 | M18 Distribution | 1, 2, 12 |
| M7 Tests | 10 | M19 Content scanning | 4, 5 |
| M8 Protected paths | 3, 6 | M20 Safe rendering | 4, 5, 9, 11 |
| M9 Dependencies | 5 | M21 Path safety | 3 |
| M10 Logging | 2, 5, 8, 12 | M22 Code that runs on its own | 6, 14 |
| M11 Competitive | Spike, skeleton, 13, 14 (measured in the pilot) | M23 Data classes | 1, 7, 9 |
| M12 Terminal interface | 11 | M24 Source control | 9, Bitbucket |
| S1 Commands in prose | 11 | S5 Fuzzing | 3, 4, 5, 10, 14 |
| S2 Guided MR review | 9, Bitbucket | S6 Validation pack | 14 |
| S3 Setup check | 11 | S7 Standing instructions | 5 |
| S4 Scripted provider | 5 | | |

Requirements divided into numbered parts are traced part by part:

| Part | Milestone | Part | Milestone |
|---|---|---|---|
| M10.1 What is logged | 5 (event interface), 8 | M10.12 Local log deletion | 2, 8 |
| M10.2 Three tiers | 8 | M10.13 Session file cleanup | 2 |
| M10.3 Keyed hashes | 8 | M10.14 Saved conversations are transcripts | 5 |
| M10.4 Wrapping the session key | 8 | M10.15 Removal (`hermit purge`) | 2, 12 |
| M10.5 Searching | 8 (`hermit-audit`) | M10.16 One repository per session | 5 |
| M10.6 Unlinking old hashes | 8 (procedure script) | M14.1 Delivery and precedence | 1, 3 (`.agentignore` only adds) |
| M10.7 Read records | 8 | M14.2 No secrets in the policy | 1, 2 |
| M10.8 Content that can't be rebuilt | 8 | M14.3 Settings, required ones enforced | 1 |
| M10.9 Log destination | 1 (validation), 8 | M14.4 Required settings | 1, 7, 8, 9 |
| M10.10 Written locally first | 8 | M14.5 Logging and retention | 1, 2, 8 |
| M10.11 Encrypted at rest | 2 | M14.6 Models and data classes | 1, 7 |
| | | M14.7 Reading, writing and scanning | 1, 3, 4, 5 |
| | | M14.8 Review and source control | 6, 9 |
| | | M14.9 Installation and operations | 1, 2, 11 |

Prerequisites P1–P12 are organizational. Milestone 14's documentation covers them, and milestone 8 supplies AWS templates for P5. P10, the independent penetration test, happens after milestone 12 and before any production pilot. P12, application allowlisting, is optional; milestone 12's signed packages and documented organization signing are what it relies on.

## 6. Open decisions

1. **Where the project is hosted,** which decides whether CI is GitHub Actions or GitLab CI.
2. **Terminal interface library:** for example Bubble Tea, or a smaller library with less to review.
3. **Which source control host comes first** for milestone 9 (assumed GitLab).
4. **Which provider comes first** for the first usable build (assumed the Claude API).
5. **Listening-process checks on macOS and Windows** (milestone 7). Linux exposes socket owners through `/proc`, and Windows through `GetExtendedTcpTable`, which Go can call without cgo. macOS has no simple Go API; the usual libraries run `lsof`, which would break the rule that only `gitcmd` runs a process, and cgo would complicate reproducible builds. The options are native system calls without cgo (Go can call libSystem functions such as `proc_pidinfo` the way its runtime does, at the cost of some low-level code), or a second, narrowly listed exception to the process rule (`lsof` with fixed arguments). Recommendation: try native calls first, and if they prove impractical, ship local-only classes on Linux and Windows first rather than weaken the rule.
6. **How `fsguard` decides whether a file is git-ignored.** This matters because ignored files are unreadable by default (M13). The options:
   - **Ask git** (`git check-ignore --stdin -z`): always matches git's behavior, including global and per-repository exclude files, but runs a process, which `fsguard` is meant to avoid.
   - **Reimplement gitignore matching:** keeps `fsguard` self-contained, but can drift from git's behavior, and a mismatch could make an ignored file readable.

   Recommendation: ask git through `internal/gitcmd`, which gives `fsguard` the answers, so `fsguard` itself still runs no processes. Hermit already requires git (M18). If git is unavailable or returns an error, treat the file as ignored, so the failure refuses rather than allows. Test against a corpus of gitignore cases to confirm the answers match git.
7. **Format of the AWS setup templates** (milestone 8, for P5): CloudFormation, CDK or Terraform. CloudFormation needs no extra tooling in an AWS-only organization; Terraform suits organizations that already manage AWS with it. Shipping one and documenting the resources well enough to translate may be enough.
8. **The macOS Keychain without cgo or running a program** (milestone 2). The common Go libraries either run `/usr/bin/security`, which breaks the process rule, or need cgo, which complicates reproducible builds. This is the same problem as open decision 5, and it is on the critical path. Windows Credential Manager and the Linux kernel keyring are reachable through system calls, and libsecret through D-Bus, without either. The options are the same as in decision 5: call Security.framework through libSystem-style trampolines without cgo, or accept cgo for the macOS build only and confirm the build stays reproducible. Recommendation: decide at the start of milestone 2, together with decision 5, since both need the same low-level calling code.
9. **The edit tool's format** (milestone 5). Options include replacing an exact, unique piece of existing text, applying a unified diff, or rewriting a whole file. The choice decides how often edits fail to apply, and how edits map onto text the model saw altered (M3). Recommendation: exact-text replacement, which current models use reliably, and which maps directly to the M3 rule: the text being replaced must appear in the model's view, and must not overlap anything redacted or removed. Confirm with the spike's findings.
10. **How developers sign in to Bitbucket Cloud** (Bitbucket support). Bitbucket Cloud documents no device sign-in. Its OAuth authorization code flow needs a browser that can reach a callback on the developer's machine, which fails on headless and remote development machines. It may also need a client secret, which can't go in the policy (M14.2). The alternative is a scoped API token that each developer creates and enters. Recommendation: API tokens, with `hermit doctor` checking their scopes. Revisit OAuth if Atlassian adds device sign-in, or confirms a flow without a client secret.
