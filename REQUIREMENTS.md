# Requirements: A Guarded Coding Agent

Status: draft for discussion.

## 1. Problem

Organizations that handle sensitive data often accept a model API (their agreements allow the data processing) but not a general-purpose coding agent such as Claude Code. Their decision makers worry about:

- **Data exfiltration:** code or data leaving through a channel nobody reviewed.
- **Reach:** the agent touching systems it shouldn't.
- **Comprehension:** developers not understanding what is being done and why.

Turning off command execution doesn't solve this. The agent then suggests commands for the developer to run, and those are often chained pipelines that are hard to read. The developer becomes the checkpoint, and that checkpoint gets weaker as they get tired or a deadline gets closer. A command copied out of a chat also bypasses every control: it runs in the developer's own shell, with their full environment and credentials, and nothing records that a model suggested it.

Manual approval of each command has the same weakness. Safety ends up depending on the developer's patience.

## 2. Goal

> Every action that takes effect is one the developer understood and intended, and the design does not depend on the developer's patience to make that true.

A second goal carries equal weight: **the tool must be convenient enough that developers don't go around it** to an unrestricted consumer product. A safe tool nobody uses provides no safety. Convenience is a security requirement.

## 3. Design principles

1. **Capabilities by construction.** The agent can do only what the harness offers as a named, structured action. There is no general shell to restrict, because there is no shell.
2. **Fewer decisions, each one cheap.** Reading and searching need no approval. Approvals are reserved for changes, and each one is small enough to actually review.
3. **Deterministic descriptions.** When the developer is asked to approve something, the harness's code describes it, not the model. The model's own explanation can be wrong or the product of a prompt injection.
4. **Reversible by default.** Changes go to a branch and are reviewed as diffs. The only way out of the developer's machine is a merge request, which another person reviews.
5. **Understanding through normal review.** Comprehension comes from reviewing a plan and small diffs, not from quizzes or attestations that people would resent and skip.
6. **The harness enforces, the model is instructed.** Instructions to the model are a courtesy. Every hard rule is enforced in harness code.

## 4. Users

- Mostly experienced engineers. Less experienced developers may also be given access.
- Everyone, including experts, will at times accept suggestions without reading them. The design limits what such approvals can do.
- Developers use different IDEs, mainly Eclipse and VS Code.

## 5. Threat model

| Threat | Example | Main mitigations |
|---|---|---|
| Honest model mistake | Deletes the wrong code, misreads requirements | Plan approval, small reviewed diffs, branch-only changes, MR review |
| Prompt injection | Text in the repository or a dependency instructs the agent | No shell, no network, no credentials; fixed actions only; protected paths |
| Data exfiltration | Agent sends code or secrets somewhere | No network access for the agent; only outbound channels are the model API and the organization's own GitLab |
| Reaching other systems | Agent queries a database or calls an internal service | No shell and no credentials, so no capability to do it |
| Side door through CI | Agent-written code reads CI secrets or calls the network when the pipeline runs | Protected paths; no protected variables or deployment credentials on agent branches; restricted runners |
| Supply chain | Agent adds a malicious or typo-squatted dependency | Agent cannot edit dependency manifests; it suggests, the developer types |
| Human rubber-stamping | Developer approves everything near a deadline | Blast radius of approvals is limited to a branch and an MR that another human reviews |

## 6. Tasks in scope

**Core**

1. Break a project, such as a feature, into phases.
2. For each phase, produce a diff a human can review.
3. Write and update tests, as a phase of their own when that helps.
4. Edit files, then submit a merge request.
5. Revise the work based on MR review feedback.

**Bonus**

6. Guided MR review: walk a reviewer through someone else's MR.
7. Repository orientation: help a developer learn an unfamiliar codebase.

## 7. Requirements

### Must

**M1. Capabilities.** The agent can only:
- read and search files in the repository;
- propose changes to files;
- read MR comments and CI pipeline results;
- after developer approval, commit, push the task branch, and open or update an MR on the organization's self-hosted GitLab.

**M2. No shell, network or credentials.** The agent has no shell, no general network access, and never sees a credential. The harness holds the GitLab token and uses it only for the fixed actions in M1.

**M3. Shared working tree.** The agent works in the developer's normal working tree, alongside their uncommitted changes, in both directions: the developer can start work and have the agent finish it, or add their own touches after the agent's work. The harness:
- snapshots the tree before each agent step, so the review shows only the agent's changes for that step and the audit log can always tell the developer's work from the agent's;
- refuses a patch to any file that changed after the agent last read it, and has the agent re-read it first, so a developer's newer edits are never overwritten.

**M4. Plan first, small diffs.** The developer approves a phased plan before any code changes. Each phase's diff stays under a reviewable size limit, and larger phases are split. Each diff is linked to the plan step it implements.

**M5. Review in the IDE.** Proposed changes appear in the working tree, so developers review them with their IDE's own diff and source-control views and can edit them directly. Approving a phase commits it.

**M6. Tests.** The agent can write and update tests.

**M7. Protected paths.** The agent cannot change CI configuration, build scripts or dependency manifests. The harness refuses such patches before anything is written. There is a default protected list (for example `.gitlab-ci.yml`, `Makefile`, `Dockerfile`, `pom.xml`, `build.gradle`, `package.json`, lockfiles, `pyproject.toml`, `requirements*.txt`), and each repository can add to it in a configuration file the agent also cannot edit.

**M8. Dependencies are suggested, not added.** When a change needs a new dependency, the agent describes it (package, version, purpose) and the developer adds it by hand. It is presented as a description, not a copyable command, so adding it is a deliberate act.

**M9. Audit log.** Every action, approval and refusal is logged with the user, session and time, and the log can be exported for audit review.

**M10. Competitive on the core loop.** For plan → diffs → tests → MR → revise, the tool is fast and capable enough that developers don't want to switch to an unrestricted product.

**M11. Terminal interface.** The conversation happens in a terminal, so it works alongside any IDE. No IDE plugin is required.

### Should

**S1. Commands in prose.** The harness detects shell commands the model writes in its replies and flags them as unverified. The model is instructed to describe goals rather than write commands for the developer to run.

**S2. Guided MR review** (task 6).

### Could

**C1. Repository orientation** (task 7).

## 8. Organizational prerequisites

These aren't features of the tool, but the design depends on them:

- **P1.** In every GitLab project the agent works on, CI/CD variables containing secrets are marked *protected*, so they are available only on protected branches. Agent task branches are never protected.
- **P2.** Pipelines for agent branches run on runners with restricted network access and no deployment credentials. Agent-written tests run in CI, and test setup code can execute anything the runner allows. This is why P1 and P2 are essential even with M7.
- **P3.** MRs from the agent go through the same human review as any other MR.
- **P4.** Model API access is approved under the organization's data-processing agreements.

## 9. Success measures

To be agreed with leadership before a pilot:

- **Retention:** pilot users keep using the tool, and report (in an anonymous survey) that they don't go around it.
- **Speed:** time from feature start to MR, compared with working without the tool.
- **Quality:** rework requests and defects found in MR review, compared with MRs written without the tool.
- **Understanding:** reviewers rate, in one quick step, whether the MR author could explain the change.
- **Friction:** approvals per task, and how often protected-path refusals or dependency suggestions get in the way of legitimate work.

## 10. Out of scope

- Running commands, builds or tests on the developer's machine. Verification happens in CI.
- Changes to CI configuration, build scripts or dependencies (see M7 and M8).
- Unattended or scheduled operation. A developer is present for every task.
- Access to any system other than the repository, the model API and the organization's GitLab.

## 11. Open questions

1. **Implementation language and distribution:** how developers install and update it on macOS, Windows and Linux.
2. **Model access:** the vendor's API directly, or through a cloud provider (Bedrock, Vertex) under an existing agreement.
3. **GitLab credentials:** a per-user token (MRs attributed to the developer) or a project token; where the harness stores it.
4. **Diff size limit:** what counts as reviewable, and whether it is set per repository.
5. **CI round trip:** whether pipelines are fast enough for the agent to wait on test results, and what to do in repositories with no tests or CI.
6. **Audit log destination:** local files, the GitLab MR itself, or a central log service.
7. **Detecting commands in prose (S1):** how reliable detection can be, and what the developer sees.
8. **Eclipse users:** whether a terminal plus Eclipse's Git views is comfortable enough in practice.
