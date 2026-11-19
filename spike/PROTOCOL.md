# Spike protocol

This protocol fixes how the usefulness spike runs and how its results are judged, before any session starts. It expands the spike section of [IMPLEMENTATION_PLAN.md](../IMPLEMENTATION_PLAN.md) ("Spike: is the constrained agent useful?"), which sets the repositories, tasks, measures and decision rules. Where this protocol has to settle something the plan leaves open, the choice is listed under [Choices this protocol makes](#choices-this-protocol-makes), for the project maintainer to confirm before the first session.

Requirements: M11, and the constraints that most limit usefulness: M1, M2, M7, M8 and M13.

Status: draft. It is frozen once the project maintainer confirms it. Any change after that goes under [Amendments](#amendments), dated and with its reason. A change never applies to sessions that already ran.

## Roles

- **Project maintainer:** confirms this protocol, prepares or approves each repository's task set, makes the judgement calls the decision rules leave to them, and reads the findings.
- **Facilitator:** sets up each session, records the sheet, and doesn't help with the task. The maintainer may facilitate.
- **Participants:** two or three developers, at least one of whom didn't write the requirements. Each participant works on every task (see [Assignment](#assignment)).

## Repositories

The plan's criteria apply. There are three repositories, one each for Java (Maven or Gradle), Python (pytest) and TypeScript (Jest), each with 10,000–100,000 lines and passing tests that write JUnit XML reports. They are either public open-source repositories with nothing sensitive, or repositories an organization owns, used under the plan's conditions. The steps below marked *organization's repositories* apply only to the second kind.

For each repository, before any session:
1. *Organization's repositories:* record on the instantiation sheet the organization's written approval: the API account it covers, the P4 terms it confirmed, and whether it approved prompt caching and the server-side fallback. Any feature it didn't approve is turned off for every session.
2. Pin a commit. Every session on that repository starts from a clean clone at that commit, plus the task's prepared branch, if it has one.
3. Run the full test suite with JUnit XML reports turned on. Check that the tests pass, that the reports land where `read_test_results` finds them (or note the `-reports` glob to use), and record how long the run takes.
4. Check for anything sensitive: credentials in files, internal hostnames, personal data. The spike hides secrets files by name only and doesn't scan file contents. For a public repository, anything found rules it out.
5. *Organization's repositories:* after the tests have run, run a secret scanner over the working tree the sessions will use, tracked and untracked files, test reports included. Use the scanner the organization uses, or else an open-source one such as gitleaks in directory mode. The repository is used only if the scanner finds nothing, or if everything it finds is removed from that working tree. Rerun the scanner on each prepared branch. Record the scanner, its version and the result on the instantiation sheet.
6. Give it a code (A, B or C). The findings use the code; the instantiation sheet maps codes to repositories.

## Tasks

The plan fixes ten task types. This table spreads them across the repositories; the maintainer may swap tasks between repositories during instantiation, before the first session, when a repository can't support a task (for example, it has no generated code for task 9).

| # | Task | Repository | Kind |
|---|------|-----------|------|
| 1 | Fix a failing test caused by a seeded bug | Java | ordinary |
| 2 | Implement a small feature touching 3–5 files, with tests | Python | ordinary |
| 3 | Add tests for a module that has none | TypeScript | ordinary |
| 4 | Rename or refactor something used across the codebase | Java | ordinary |
| 5 | Revise a prepared branch in response to pasted review comments | Python | ordinary |
| 6 | A task that needs a new dependency | TypeScript | ordinary |
| 7 | A task whose natural fix touches a protected file | Python | probe |
| 8 | Explain how a subsystem works to a developer new to it | TypeScript | ordinary |
| 9 | A task that needs generated or git-ignored code to understand | Java | probe |
| 10 | A task where history would help | TypeScript | probe |

### Instantiating a task

For each task, the maintainer writes the following on the instantiation sheet, which is kept off-repo:
- **Task text:** exactly what the agent receives, saved as the `-task-file`. It is written as a developer would write it, with no hints about Hermit's constraints.
- **Setup:** the pinned commit, the prepared branch if any, and anything the participant must do first.
- **Completion criteria:** what a commit-ready result must do for this task (see below).
- **For probes and task 6:** the behaviour expected from the agent, and what alternative would count as acceptable.

Every participant gets the same task text for the same task.

### Task-specific preparation

- **Task 1, the seeded bug.** Someone who isn't a participant chooses a function covered by existing tests and introduces one plausible bug: an off-by-one error, an inverted condition, a wrong default or a missed case. The bug makes at least one test fail, and the failing test isn't in the file that holds the bug, so the agent has to trace the failure. The seeded change is committed on a prepared branch, recorded on the instantiation sheet, and not shown to participants. Participants run the tests once before giving the task to the agent, so a failing report exists.
- **Task 5, the prepared branch.** Someone who isn't a participant prepares a branch with a modest change (around 50–150 changed lines) and writes three to five review comments, as a reviewer would, including at least one that needs a change in a second file. The participant pastes the comments in.
- **Task 6.** The task needs a library the repository doesn't use yet. The agent can't edit the manifest (M8), so the expected outcome is a description the developer can act on: which package, which version constraint and why, plus any code changes, which the agent may make.
- **Task 7.** The task's natural fix changes a protected file, such as a test runner setting in `pyproject.toml`. The expected outcome is the agent explaining the change for the developer to make.
- **Task 9.** Understanding the task needs code the agent can't see: generated sources, or git-ignored build output. The expected outcome is the agent asking for what it needs, or working from what it can see and saying what it couldn't check.
- **Task 10.** A question such as "why was this check added, and is it still needed?" about code with a real history. The expected outcome is the agent answering from the code and asking the developer for history (for example, a pasted `git log` or blame excerpt).

### Completion criteria

A task is **completed** when its result meets the criterion for its kind:
- **Tasks 1–5:** a diff the participant would commit with at most minor edits, with the tests the participant runs passing. Minor edits are cosmetic changes, comments or naming, not changes to behaviour.
- **Task 6:** the description of the dependency change is correct and enough for the participant to make it, and any code changes meet the criterion for tasks 1–5.
- **Task 8:** the participant judges the explanation accurate, after spot-checking at least two of its claims against the code, and says it would have saved them time.
- **Probes (7, 9, 10):** the plan doesn't count probes towards completion. The record is whether the agent's alternative was acceptable to the participant.

## Assignment

Each participant attempts every task, so there are 20 sessions with two participants and 30 with three.

Each session uses one edit format. Formats rotate so that each participant uses every format and each task is tried in different formats. With formats numbered replace = 0, diff = 1 and whole = 2, participant *p* (numbered from 0) uses format (*task* + *p*) mod 3. With three participants, every task is tried in all three formats.

Participants work through the repositories in a rotated order (A, B, C; then B, C, A; then C, A, B), with tasks in numeric order within each repository. A participant who already knows a repository well says so, and it's recorded.

## Running a session

1. **Briefing.** Each participant gets the same short briefing once, before their first session: what the agent can and can't do, how to run the tests with reports, and the commands (`/stats`, `/done`, Ctrl-C). On an organization's repositories, the briefing also says to paste nothing from production systems: no production logs, data or credentials. Nobody coaches them on how to write prompts.
2. **Setup.** Start from a clean clone at the pinned commit. Check out the prepared branch if there is one, and check that `git status` is clean.
3. **Start.** Run the spike with `-task`, `-who` (the participant's code), `-format` (from the rotation) and `-task-file`. The facilitator notes the start time.
4. **Work.** The participant works as they would with their usual tool: they reply to the agent, run tests when they choose, and paste anything the agent asks for, if they're willing to. Each test run is tallied on the sheet. The facilitator steps in only for technical problems, such as a crash or an API error, and records any intervention.
5. **Limit.** A session ends after 60 minutes. A task that isn't completed by then counts as not completed.
6. **End.** The participant types `/done` when the result is commit-ready or they give up. The facilitator then records the sheet with the participant (see below), before the next session.
7. **Keep the result.** Save `git diff` from the session's working tree with the log, so completion can be checked later.

A session that a crash or an API outage ends early is rerun from the start in the same format, and the failed session is recorded as voided. A session voided for any other reason is recorded with the reason.

## Recording sheet

Record one row per session. The fields marked *log* come from the session summary at the end of the log. The others are recorded by the facilitator with the participant.

| Field | Values |
|-------|--------|
| Session ID | log file name |
| Participant | code |
| Repository, task | code, 1–10 |
| Edit format | replace, diff or whole |
| Started, ended | times |
| Elapsed, model turns, developer messages | *log* |
| Edits attempted, edits failed by reason | *log* |
| Refusals by kind | *log*: blocked attempts only |
| Limits the agent mentioned | the agent's statements that it couldn't do something without trying, such as "I can't edit that file" |
| Developer test runs | tally |
| Completed | yes, or no with the main reason |
| Edits before committing | none, minor or substantial |
| Can the participant explain the diff? | yes, partly or no: asked to explain it in two or three sentences without looking at the agent's explanation |
| Rating | 1–5: "Would you use this instead of your usual tool for this task?" |
| Blocked by a constraint? | none, protected file, ignored or generated file, no history, no shell, or other: the participant and facilitator agree whether a constraint was the main reason for a failure or detour |
| Agent's alternative acceptable? (probes and task 6) | yes, no, or not offered |
| Facilitator interventions | none, or what happened |
| Notes | free text |

## Applying the decision rules

The rules are the plan's, applied as written. Where the plan's rule is stated per task and a task has several attempts, the following counting applies.

- **Task completion.** A task counts as completed when more than half its attempts completed. A task with exactly half is reported as split and counts as not completed.
- **Proceed, stop or decide.** Count the completed ordinary tasks (1–6 and 8). Five or more: proceed as specified, unless another rule triggers. Exactly four: the maintainer decides and the findings record why. Fewer than four: stop and revisit the constraints before milestone 1, with the question in `QUESTION.md`.
- **Agent-run tests.** Take the median of developer test runs across the attempts at task 1. If it exceeds 4, raise agent-run tests as a requirements question.
- **M8 and M13 defaults.** If any attempt at tasks 1–6 or 8 was blocked by a protected file, an ignored file or missing history, revisit the defaults. For each probe, the alternative counts as acceptable if a majority of its attempts recorded it as acceptable. If two or more probes don't count as acceptable, revisit the defaults.
- **Edit format.** Each format's failure rate is its failed edits divided by its attempted edits, pooled across all sessions. Choose the format with the lowest rate. If every format's rate is over 10%, the edit tool needs more design work before milestone 5. The findings report the counts with the rates. If the lowest two rates are equal, the maintainer chooses.

The findings record every measure for every session, the per-task results, and each rule's outcome with the numbers behind it.

## Data handling

- The logs record developer messages, the agent's text and the paths it touched; file contents appear only where quoted in that text. Logs stay on the participant's machine, under `~/hermit-spike-logs`, until the facilitator collects them, and are never committed.
- The instantiation sheet, the recording sheets and the saved diffs stay off-repo.
- The findings in `docs/spike-findings.md` use repository and participant codes, and contain no task text or code beyond what's needed to explain a result.
- The API key comes from each participant's own secret store, through `HERMIT_SPIKE_ANTHROPIC_KEY` at launch only. It is never written to a file or pasted into a session.

For an organization's repositories, these also apply:
- Sessions use only the API account the organization's approval covers.
- Participants are people already allowed to read the repositories, working on machines the organization manages.
- Logs, saved diffs and both sheets are confidential to the organization and stay on its machines or storage. They are deleted 30 days after the project maintainer has read the findings, unless the organization's retention rules say otherwise. The facilitator records the deletion date.
- The findings name no repository or organization and contain no task text, code or file paths. The organization reads them before they're committed.
- The tasks stay with the organization. Milestone 13's evaluation suite gets tasks of the same ten types on public repositories instead.

## Dry run

Before the first counted session, the facilitator runs one task end to end on a repository outside the set, to check the setup, the reports and the sheet. The dry run isn't counted. A smoke test on throwaway repositories (2026-11-19) checked the prototype itself in all three formats.

## Choices this protocol makes

The plan leaves these open. The maintainer confirms or changes them before the first session:
1. Every participant attempts every task.
2. Edit formats rotate by (task + participant) mod 3.
3. Tasks are spread across the repositories as in the table under [Tasks](#tasks).
4. Each session ends after 60 minutes.
5. A task with several attempts counts as completed when more than half of them completed; an exact split counts as not completed.
6. A probe's alternative counts as acceptable when a majority of its attempts recorded it as acceptable.
7. Any single blocked attempt at an ordinary task triggers the M8 and M13 rule.
8. Edit failure rates are pooled across sessions, and a tie between the two lowest rates goes to the maintainer.
9. Limits the agent mentioned without trying are recorded alongside the logged refusals. The smoke test showed that the agent often states a limit from its instructions instead of trying, so refusal counts alone undercount how often constraints come into play.

## Amendments

None yet.
