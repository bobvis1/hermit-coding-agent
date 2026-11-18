# Implementation Plan: Hermit

Status: draft. The specification is [REQUIREMENTS.md](REQUIREMENTS.md); requirement IDs (M1–M24, S1–S7, P1–P11) refer to it. Milestones are numbered 0–13 and are never written with an "M", to keep them distinct from requirement IDs.

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
| `internal/localstore` | Encrypted local storage for everything the harness keeps on the developer's machine (snapshots, pending changes, conversation state, the local log), with its key in `credstore`; retention and cleanup | M10 |
| `internal/fsguard` | The only code that touches the repository's files: root confinement, path normalization, protected, unreadable and allowlisted paths, file-type detection, atomic and conditional writes | M8, M13, M21, M6 |
| `internal/scan` | Secrets, organization patterns, personal data, health pack, reserved test values | M19 |
| `internal/render` | Escape stripping, hidden and bidirectional characters, link and image handling, on-screen masking | M20 |
| `internal/tools` | The agent's fixed tool set, each tool a thin layer over `fsguard`, `scan`, `scm` and `testresults`; tracks what the agent has read so stale edits are refused | M1, M2, M3, M9 |
| `internal/loop` | The agent loop: messages, tool calls, condensing long conversations, standing instructions | M1, M11, S7 |
| `internal/provider` | Provider interface and adapters: Anthropic, Bedrock, OpenAI-compatible, gateway, scripted | M17, S4 |
| `internal/worktree` | Snapshots, undo, phase file lists, pending-changes mode | M3, M4, M22 |
| `internal/testresults` | JUnit XML parsing, freshness, detail levels | M15 |
| `internal/scm` | Fixed source control calls for GitLab and GitHub | M24, S2 |
| `internal/audit` | Log tiers, keyed hashes, hash chain, upload, anchoring, on top of `localstore` | M10, M16 |
| `internal/netclient` | The one HTTP client for every outbound connection: proxy, extra CA certificates, HTTPS except on loopback | M14, M17 |
| `cmd/hermit-audit` | Organization-run tooling, not on developer machines: log cross-checks, anchor checks, alerts, investigator search | M16 |
| `internal/tui` | Terminal interface on the alternate screen | M12, M20, S1 |

**Required paths.** Four rules carry most of the security, so each is enforced by the compiler or by CI, not by convention:

| Rule | How it is enforced |
|---|---|
| Every repository read and write goes through `fsguard` | Dependency check in CI: only `fsguard` may call the file system on repository paths |
| Every byte sent to the model passes through `scan` | The provider interface accepts only `scan.Cleared`, a type with unexported fields that only `scan` can construct. `provider` may not import `fsguard`, `tools`, `scm` or `testresults`, so it has no other source of content |
| Every byte displayed, and every MR text sent, passes through `render` | `tui` accepts only `render.Safe`, and `scm` accepts MR text only as `render.ApprovedMRText`, which is created only by the approval step (M20) |
| Every tool call is logged | Tools run only through one wrapper in `loop` that records the call before and after. A test enumerates every tool and checks that calling it produces audit events |

Other dependency rules, also checked in CI from milestone 0:
- `fsguard`, `scan` and `render` import nothing from the rest of Hermit.
- Everything stored on the developer's machine goes through `localstore`.
- Every outbound network connection goes through `netclient`; no other package imports `net/http` directly.

## 2. How each milestone is run

1. **Start a fresh session** for the milestone. Name it after the milestone so it can be resumed.
2. **Point at the spec.** For example: *"Implement milestone 3 from IMPLEMENTATION_PLAN.md. The requirements are M21, M8, M13 and M6 in REQUIREMENTS.md. Write the failing tests for every case listed in M21 and S4 first, then implement until `make check` passes."*
3. **Tests first for security rules.** For `policy`, `localstore`, `fsguard`, `scan` and `render`, the hostile cases from the requirements are written as failing tests before the implementation.
4. **Run the checks.** The Stop hook (milestone 0) runs `make check` whenever the agent tries to finish with Go changes, so a milestone can't end with failing tests. Ask for the command output as evidence, not a summary. If the agent needs to stop and ask instead (below), it writes the question to `QUESTION.md`, which lets it stop.
5. **Review against the spec.** The main session saves the milestone's diff to `review/milestone.diff` (gitignored), then asks the read-only `spec-reviewer` subagent to compare that file with the milestone's requirements: every requirement implemented, every listed case tested, gaps that affect correctness only, no style comments. The reviewer reads the diff from the file, so it needs no shell.
6. **A developer reviews and commits.** Nothing merges without a person reading the diff.
7. **Clear between milestones.** Unrelated history makes later sessions slower and more expensive.

**Effort.** Use high effort for milestones 1, 2, 3, 4, 8 and 9, where a missed edge case is a security bug: fail-closed policy loading, local storage and credentials, the file access guard, scanning and rendering, logging, and the source control limits. Default effort is enough elsewhere.

**Parallel work.** Some milestones are independent (section 4). Run them as separate sessions in separate worktrees so their edits can't collide, and merge them one at a time.

**When to stop and ask.** If the requirements turn out to be wrong, ambiguous or contradictory, the session writes its question to `QUESTION.md` and stops (the Stop hook allows this, milestone 0). Changes to REQUIREMENTS.md are made by a person, never as a side effect of implementing a milestone.

## 3. Milestones

Each milestone lists what it delivers and its checks: what must pass before it counts as done.

### Milestone 0: Repository and checks

Delivers:
- Go module pinned to Go 1.25 or later, in `go.mod` and in CI. `fsguard` needs `os.Root.Rename` for atomic writes inside the root, and the hostile tests need `os.Root.Link` to create hard links; both arrived in Go 1.25. (Go's FIPS mode, mentioned in M18, has been available since 1.24, so 1.25 covers both.)
- Package layout from section 1, `Makefile` with `make check` (format check, `go vet`, `staticcheck`, unit tests, the dependency-rule checks) and `make fuzz`.
- CI running `make check` on macOS, Windows and Linux (S4 requires all three).
- Development guardrails for building Hermit with Claude Code:
  - a short `CLAUDE.md` (draft below);
  - `.claude/settings.json` with the `Stop` hook below; `ask` for edits to `REQUIREMENTS.md`; `deny` for reading credential files;
  - `.claude/agents/spec-reviewer.md`: tools limited to `Read, Grep, Glob`, instructed to compare the diff file it is given against named requirements and skip style;
  - `.gitignore` entries for `review/` and `QUESTION.md`.

**The Stop hook** (`scripts/stop-check.sh`) blocks the agent from finishing while `make check` fails, with two exceptions:
- **No Go changes:** if no `.go` files, `go.mod`, `go.sum`, test fixtures or the `Makefile` changed in the working tree, it allows the stop, so documentation-only sessions aren't held up.
- **A question is pending:** if `QUESTION.md` exists, it allows the stop and shows the question to the developer. This is how "stop and ask" (section 2) works when the requirements are ambiguous. The developer answers, deletes the file and resumes the session.

It does not use `stop_hook_active` as an exception, because that would let the agent stop after a single retry with tests still failing. Claude Code's own limit on repeated blocks prevents an endless loop. Using the question file to get past failing tests is visible to the developer, who sees the question.
  - `.claude/rules/security-core.md` with `paths:` set to the security packages, holding the conventions for that code (below).

Checks: `make check` passes on all three operating systems in CI; the Stop hook blocks a deliberately failing test, allows a documentation-only change, and allows a stop when `QUESTION.md` exists; each dependency rule fails on a deliberate violation.

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

### Milestone 1: Policy

Requirements: M14, M18 (policy location, refuse writable policy, fail closed, managed-device detection, recheck before each request, unmanaged mode, minimum harness version), M23 (class ranking and repository matching). High effort.

Delivers: `internal/policy` with schema validation of the full settings table in M14, per-OS policy locations, the minimum-version check, and data class resolution from the git remote.

Checks:
- A harness older than the policy's minimum version refuses to start.
- A malformed, unreadable or developer-writable policy stops the harness.
- A missing policy on a device with a managed marker stops the harness; with no marker, unmanaged mode.
- An unmatched repository gets the most restrictive ranked class.
- A changed policy file is picked up by the recheck; `enabled: false` stops the next request.
- The example policy in REQUIREMENTS.md validates.

### Milestone 2: Credentials, local storage and network client

Requirements: M18 (credential storage and fallbacks), M10 (files on the developer's machine, session maximum age), M14 (HTTPS proxy and extra CA certificates), M17 (HTTPS except on loopback). High effort. Can run in parallel with milestones 3 and 4.

Delivers: `internal/credstore` (operating system credential store, kernel keyring on headless Linux, sign in each session); `internal/localstore` (encryption at rest with its key in `credstore`, readable only by the developer's account, retention, session maximum age and cleanup rules from M10, removal on uninstall); and `internal/netclient`, the one HTTP client every outbound connection uses (model providers, source control, log upload), applying the proxy, extra CA certificates and the HTTPS-only rule.

Checks:
- `netclient` refuses plain HTTP except to loopback addresses, and uses the configured proxy and CA certificates (tests against local test servers with a private CA).
- Nothing written through `localstore` is readable as plaintext on disk (test reads the raw files).
- Files are created with owner-only permissions on each operating system.
- Retention and cleanup: session files removed at session end or maximum age; local logs removed only after confirmed upload plus the retention period.
- Each `credstore` backend round-trips a secret, and "sign in each session" stores nothing.

### Milestone 3: File access guard

Requirements: M21, M8, M13, M6. High effort. Can run in parallel with milestones 2 and 4.

Delivers: `internal/fsguard`, the single gate for repository file access: root-confined operations (`os.Root`), resolved-path rule checks, case- and Unicode-insensitive matching, Windows name rules, multiply-linked file refusal, `.git` in every form, atomic writes, a conditional write that succeeds only if the file still matches a given content hash (used for stale-edit detection in milestone 5), protected and unreadable lists loaded from per-ecosystem data files, read and write allowlists, git-ignored files, binary and container detection by content, notebook output stripping, and refusal of hard-to-review writes.

Checks:
- Every M21 case from S4's list, as tests on all three operating systems.
- A conditional write fails if the file changed after the given hash was taken.
- Refused reads return the same "not available" result whether or not the file exists, and unreadable names never appear in listings or search.
- Fuzz targets for path normalization and matching run in CI.

### Milestone 4: Scanning and rendering

Requirements: M19, M20. High effort. Can run in parallel with milestones 2 and 3.

Delivers: `internal/scan` (secrets, organization patterns, opt-in personal data and health pack, reserved test values, redact and block modes) and `internal/render` (escape stripping, hidden and bidirectional characters, link and image handling for MR text and markdown, masking).

Checks:
- Table-driven tests from fixture files: known token formats, HL7v2, FHIR, DICOM marker, check-digit identifiers, reserved test ranges not flagged.
- Escape-sequence and Unicode fixtures, including Trojan Source examples and OSC 8 links.
- Fuzz targets for the scanner and the renderer.

### Milestone 5: Agent loop and scripted provider

Requirements: M1, M2, M9, M3 (stale-edit detection), M10 (one repository per session, resume rules, condensed summaries as transcripts), S4, S5 (tool-call parser), S7. Depends on milestones 2, 3 and 4.

Delivers: `internal/tools` (the fixed tool set, no shell or network tools; read tracking and stale-edit refusal using `fsguard`'s conditional write), `internal/loop` (with every model-bound message passing through `scan` and every displayed message through `render`; one repository per session; conversation state and condensed summaries kept in `localstore` as local transcripts; resume only within the session maximum age), the provider interface, including a hook for per-request metadata (milestones 7 and 8), and the scripted provider. Standing instructions (S7).

Checks:
- Scripted hostile sessions from S4 (write `.git/hooks/pre-commit`, read `.env`, overwrite a newer developer edit, malformed tool calls) are all refused, with the refusal recorded.
- A planted secret in a file the agent reads reaches the scripted provider redacted.
- The required paths in section 1 hold: code that passes a plain string to a provider doesn't compile, and every tool in the enumerated tool set produces audit events when called.
- No tool exists that runs a command or opens a network connection: a test enumerates the tool set.
- The scripted provider can't be enabled through organization policy.
- A session can't switch repositories, and resuming after the session maximum age is refused.
- A fuzz target for the tool-call parser runs in CI (S5).

### Milestone 6: Working tree

Requirements: M3 (snapshots and undo), M4, M5, M8 (flagging edits to scripts CI calls), M22.

Delivers: `internal/worktree`: snapshots in `localstore`, `hermit undo` including untracked files, phase file lists and the unplanned-edit report, sensitive-path notes, phase size warnings, pending-changes mode, and flags in the change summary for edits to scripts that CI configuration references (found by reading known CI formats through `fsguard`).

Checks: scripted sessions that edit, delete and create files, then undo; the harness never runs `git commit` or `git push` (a test enumerates git invocations); an edit to a script referenced from GitLab CI and GitHub Actions fixtures is flagged, not refused.

### Milestone 7: Model providers and data classes

Requirements: M17, M23 (provider and feature gating, local-listener verification, repository identity and class sent to the gateway), M14 (usage limits). Can run in parallel with milestones 6, 8 and 9.

Delivers: adapters for the Claude API, Bedrock, OpenAI-compatible endpoints and the organization gateway, using `credstore` for keys and tokens and `netclient` for connections; approved-model and feature enforcement per class; listening-process check for local-only classes; usage limits per session and per day, counted from provider responses and kept in `localstore`; per-request metadata sent to the gateway. This milestone owns the metadata mechanism and the repository identity and class it carries; milestone 8 adds the log chain head to it.

Checks:
- Adapter contract tests against recorded responses, no live calls in `make check`.
- An unapproved model, cross-region profile or feature is refused before any request.
- A loopback port held by `ssh` or an unidentified process is refused for a local-only class.
- Requests stop with a clear message once the session or daily usage limit is reached.
- Gateway requests carry the repository identity and class (recorded-request test).
- A manual smoke test against a local model (Ollama), as described in S4.

### Milestone 8: Logging

Requirements: M10, M16. High effort. Can run in parallel with milestones 6, 7 and 9.

Delivers:
- `internal/audit` on top of `localstore`: three tiers, keyed hashes with the organization KMS key, read records (blob hash and line range), escrow option, condensed summaries logged as transcripts, hash chain, anchoring (the chain head added to each gateway request through milestone 7's metadata mechanism, or uploaded to S3 every minute without a gateway), batched S3 upload with retry.
- `cmd/hermit-audit`, separate tooling the organization runs on its own infrastructure, not on developer machines: scheduled cross-checks of Hermit's logs against the gateway's request log, CloudTrail or provider usage records, checks that uploaded logs reach their anchors, alerts on gaps, and investigator search by keyed hash.
- AWS setup templates for the bucket, keys and roles in M16 (for P5), and for running the cross-checks.

Checks:
- Cross-checks flag model usage with no matching session, and a session whose uploaded log stops short of its anchor (tests against fixture records).
- No plaintext path, query or argument appears in an audit event (property test over generated sessions).
- Truncating the local log is detected against the anchor.
- Upload tests against a local S3-compatible server; a separate manual test against a real AWS account, verifying Object Lock modes and crypto-shredding.

### Milestone 9: Source control

Requirements: M24, S2, M20 (MR text approval and traceability). GitLab first, then GitHub. High effort. Can run in parallel with milestones 6, 7 and 8.

Delivers: `internal/scm` with the fixed call list, current-project check, branch rules, diff filtering by read rules, host content passed through `scan` and `render`, exact-text approval with session ID and phases, and retrieval of CI test reports.

Checks: tests against recorded API responses; a test enumerates the client's endpoints and fails if one outside the fixed list appears; MR diffs touching unreadable files are filtered; MR text can be sent only as `render.ApprovedMRText`.

### Milestone 10: Test results

Requirements: M15, M7. Depends on milestone 9 for CI results.

Delivers: `internal/testresults`: JUnit XML parsing with size limits and no external entities, freshness against snapshots, detail levels per class, and summaries of CI test reports retrieved through `scm`.

Checks: fixtures from Maven, Gradle, pytest and Jest; malformed and oversized XML; a fuzz target.

### Milestone 11: Terminal interface and setup check

Requirements: M12, S1, S3, M20 (display).

Delivers: `internal/tui` on the alternate screen, plan approval, phase summaries, flagged commands in prose, paste handling per class, the startup class summary, support details from the policy shown at startup and in every error; `hermit doctor`.

Checks: rendering tests from recorded sessions; every error path shows the support contact; `hermit doctor` reports each failure with the setting to fix; a usability walkthrough in Eclipse's Terminal view and VS Code's integrated terminal.

### Milestone 12: Packaging and distribution

Requirements: M18.

Delivers: reproducible builds, signed and notarized macOS package, signed MSI, deb and rpm, SBOM and build provenance, FIPS build, MDM profile and ADMX templates, documented organization signing.

Checks: two independent builds produce identical executables; packages install and uninstall cleanly on each OS, and uninstall removes everything (M10).

### Milestone 13: Evaluation, validation and documentation

Requirements: M11, M17 (evaluation suite), S5, S6, M22 (IDE guidance), section 12 (operations).

Delivers: the evaluation suite organizations run before approving a model; the validation support pack; recommended IDE settings; administrator documentation, including the kill switch and incident runbook; pilot materials for the success measures in section 9.

Checks: the evaluation suite runs end to end against at least two providers; a reviewer who did not build Hermit can install and configure it from the documentation alone.

## 4. Order and parallel work

Milestones (not requirement IDs):

```
0 ─ 1 ─┬─ 2 ─┐
       ├─ 3 ─┼─ 5 ─┬─ 6 ────────┐
       └─ 4 ─┘     ├─ 7 ────────┤
                   ├─ 8 ────────┼─ 11 ─ 12 ─ 13
                   └─ 9 ─ 10 ───┘
```

- **Critical path:** policy, then credentials and local storage, the file access guard, and scanning and rendering, then the agent loop. The loop is where every model-bound message passes, so nothing that talks to a model is built before `fsguard`, `scan`, `render` and `localstore` exist.
- **Parallel tracks:** milestones 2, 3 and 4 touch separate packages and can run at the same time. So can milestones 6, 7, 8 and 9. Milestone 10 waits for 9, because it summarizes CI results retrieved through `scm`.
- **First usable build:** after milestones 0–6, plus a minimal terminal interface and a single provider, Hermit can be used on its own repository with the scripted or a local provider. Dogfooding from that point surfaces usability problems early (M11).

## 5. Traceability

| Requirement | Milestone | Requirement | Milestone |
|---|---|---|---|
| M1 Capabilities | 5 | M13 Unreadable files | 3 |
| M2 No shell, network, credentials | 5 | M14 Organization policy | 1, 2, 7, 11 |
| M3 Shared working tree | 3, 5, 6 | M15 Test results | 10 |
| M4 Plan and phases | 6 | M16 Central log storage | 8 |
| M5 Developer commits | 6 | M17 Model providers | 2, 7, 13 |
| M6 Reviewable writes | 3 | M18 Distribution | 1, 2, 12 |
| M7 Tests | 10 | M19 Content scanning | 4 |
| M8 Protected paths | 3, 6 | M20 Safe rendering | 4, 9, 11 |
| M9 Dependencies | 5 | M21 Path safety | 3 |
| M10 Logging | 2, 5, 8 | M22 Code that runs on its own | 6, 13 |
| M11 Competitive | 13 (measured in the pilot) | M23 Data classes | 1, 7 |
| M12 Terminal interface | 11 | M24 Source control | 9 |
| S1 Commands in prose | 11 | S5 Fuzzing | 3, 4, 5, 10, 13 |
| S2 Guided MR review | 9 | S6 Validation pack | 13 |
| S3 Setup check | 11 | S7 Standing instructions | 5 |
| S4 Scripted provider | 5 | | |

Prerequisites P1–P11 are organizational. Milestone 13's documentation covers them, and milestone 8 supplies AWS templates for P5. P10, the independent penetration test, happens after milestone 12 and before any production pilot.

## 6. Open decisions

1. **Where the project is hosted,** which decides whether CI is GitHub Actions or GitLab CI.
2. **Terminal interface library:** for example Bubble Tea, or a smaller library with less to review.
3. **Which source control host comes first** for milestone 9 (assumed GitLab).
4. **Which provider comes first** for the first usable build (assumed the Claude API).
5. **Listening-process checks on Windows,** which need a separate implementation; confirm it is feasible before committing to local-only classes on Windows.
6. **How `fsguard` decides whether a file is git-ignored.** This matters because ignored files are unreadable by default (M13). The options:
   - **Ask git** (`git check-ignore --stdin -z`): always matches git's behavior, including global and per-repository exclude files, but runs a process, which `fsguard` is meant to avoid.
   - **Reimplement gitignore matching:** keeps `fsguard` self-contained, but can drift from git's behavior, and a mismatch could make an ignored file readable.

   Recommendation: ask git, from a small separate package that gives `fsguard` the answers, so `fsguard` itself still runs no processes. Hermit already requires git (M18). If git is unavailable or returns an error, treat the file as ignored, so the failure refuses rather than allows. Test against a corpus of gitignore cases to confirm the answers match git.
7. **Format of the AWS setup templates** (milestone 8, for P5): CloudFormation, CDK or Terraform. CloudFormation needs no extra tooling in an AWS-only organization; Terraform suits organizations that already manage AWS with it. Shipping one and documenting the resources well enough to translate may be enough.
