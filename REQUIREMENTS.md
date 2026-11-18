# Requirements: A Guarded Coding Agent

## 1. Problem

Organizations that handle sensitive data often accept a model API (their agreements allow the data processing) but not a general-purpose coding agent such as Claude Code. Their decision makers worry about:

- **Data exfiltration:** code or data leaving through a channel nobody reviewed.
- **Reach:** the agent touching systems it shouldn't.
- **Comprehension:** developers not understanding what is being done and why.

Turning off command execution doesn't solve this. The agent then suggests commands for the developer to run, and those are often chained pipelines that are hard to read. The developer becomes the checkpoint, and that checkpoint gets weaker as they get tired or a deadline gets closer. A command copied out of a chat also bypasses every control: it runs in the developer's own shell, with their full environment and credentials, and nothing records that a model suggested it.

Manual approval of each command has the same weakness. Safety ends up depending on the developer's patience and discipline.

## 2. Goals

> Every action that takes effect is one the developer understood and intended.

> The tool is convenient enough that developers don't circumvent it with more powerful ones.

A safe tool a developer doesn't use in favor of a consumer product with no data processing agreement provides no safety. Convenience, usability and usefulness are security requirements. The tool should be competitive with tools a developer might encounter when working on less sensitive systems.

## 3. Design principles

1. **Capabilities by construction.** The agent can do only what the harness offers as a named, structured action. There is no general shell to restrict, because there is no shell.
2. **The commit is the approval.** The agent reads, searches and edits files without asking, within the limits in M7 and M12. The developer approves the plan up front, then approves changes the way they already do: by reviewing and committing them. The agent never commits or pushes.
3. **Deterministic descriptions.** When the developer is asked to approve or review something, the harness's code describes it, not the model. The model's own explanation can be wrong or the product of a prompt injection.
4. **Reversible by default.** The agent's changes stay uncommitted in the working tree until the developer commits them. Nothing leaves the developer's machine until the developer pushes, and code reaches shared branches only through a merge request, which someone else reviews.
5. **Understanding through normal review.** Comprehension comes from well-structured comprehensible diffs, not from quizzes or attestations that people would resent and skip.
6. **The harness enforces, the model is instructed.** Instructions to the model are a suggestion. Every hard rule is enforced in harness code. Do not fully trust the model to understand developer intent.

## 4. Users

- Developers of all skill levels.
- Everyone, including experts, will at times accept suggestions without reading them. The design limits what such approvals can do. Users must be instructed to remain critical.
- Developers use different IDEs and operating systems.

## 5. Threat model

| Threat | Example | Main mitigations |
|---|---|---|
| Honest model mistake | Deletes the wrong code, misreads requirements | Plan approval, small phases reviewed before the developer commits, MR review |
| Prompt injection | Text in the repository or a dependency instructs the agent | No shell, no network, no credentials; fixed actions only; protected paths |
| Data exfiltration | Agent sends code or secrets somewhere | No network access for the agent; only outbound channels are the model API, the organization's source control host and its log storage, all used by the harness, not the agent; files that must never reach the model API cannot be read (M12) |
| Local execution of agent-written files | Agent writes a git hook, IDE task or `.envrc` that runs on the developer's machine with their credentials | These paths are protected (M7), including everything under `.git/` |
| Local execution of agent-written tests | A test or its setup code reads credentials or calls the network when the developer runs it | The agent's changes are shown separately for review (M3) before the developer runs them; optional sandbox (C2) and risky-code flags (C3) |
| Reaching other systems | Agent queries a database or calls an internal service | No shell and no credentials, so no capability to do it |
| Side door through CI | Agent-written code reads CI secrets or calls the network when the pipeline runs | Protected paths; no protected variables or deployment credentials on agent branches; restricted runners |
| Supply chain | Agent adds a malicious or typo-squatted dependency | Agent cannot edit dependency manifests; it discusses trade-offs, the developer decides and types |
| Human rubber-stamping | Developer commits everything unread near a deadline | A careless commit reaches only a branch and an MR that another human reviews, not the developer's machine or other systems |

There is no reliable mitigation to developers not reviewing code. Developers must take responsibility for the MRs they author and approve.

## 6. Tasks in scope

**Core**

1. Break a project, such as a feature, into phases.
2. For each phase, make changes comprehensible for the developer to review and commit on their own.
3. Write and update tests, as a phase of their own when that helps.
4. Edit files. Once the developer has committed and pushed, open a merge request.
5. Revise the work based on MR review feedback.

**Bonus**

6. Guided MR review: walk a reviewer through someone else's MR.
7. Repository orientation: help a developer learn an unfamiliar codebase.

## 7. Requirements

**Terms.** The tool assumes Git for version control, because snapshots (M3) and commit-as-approval (M5) are built on it. It works with any **source control host** that offers merge requests and CI results through an API, such as GitLab, GitHub or Bitbucket, self-hosted or not. "MR" means a merge request, which some hosts call a pull request. **Organization policy** means settings the organization controls and developers cannot change (M13).

### Must

**M1. Capabilities.** The agent can only:
- read and search files in the repository, except the paths in M12;
- create, edit and delete files in the repository's working tree without asking, except the paths in M7;
- read MR comments and CI pipeline results;
- write an MR description, and open or update an MR on the organization's source control host for a branch the developer has pushed;
- read the latest test results from the developer's test runs (M14).

The agent never commits or pushes. The developer does both.

**M2. No shell, network or credentials.** The agent has no shell, no general network access, and never sees a credential. The harness holds the source control host token, the model credential and the log upload credential, and uses each only for its fixed purpose.

**M3. Shared working tree.** The agent works in the developer's normal working tree, alongside their uncommitted changes, in both directions: the developer can start work and have the agent finish it, or add their own touches after the agent's work. The harness:
- snapshots the tree before each agent step, so the review shows only the agent's changes for that step and the audit log can always tell the developer's work from the agent's;
- refuses a patch to any file that changed after the agent last read it, and has the agent re-read it first, so a developer's newer edits are never overwritten.

**M4. Plan first, small phases.** The developer approves a phased plan before any code changes. The agent keeps each phase small enough for the developer to review and commit on its own, and splits larger phases. The harness reports which plan step each set of changes implements.

**M5. Review in the IDE; the developer commits.** The agent's changes appear in the working tree, so developers review them with their IDE's own diff and source-control views and can edit them directly. Committing is the approval. The harness never commits.

**M6. Tests.** The agent can write and update tests. The developer runs them locally, and CI runs them on push. The agent reads the results (M14 and M1) but does not run tests itself.

**M7. Protected paths.** The agent cannot create or change files that are executed or interpreted automatically, outside a reviewed code path. The harness refuses such changes before anything is written. The default protected list covers:
- **CI configuration:** for example `.gitlab-ci.yml`, `.github/workflows/`, `Jenkinsfile`, and files they include;
- **build scripts and dependency manifests:** `Makefile`, `Dockerfile`, `pom.xml`, `build.gradle`, `package.json`, lockfiles, `pyproject.toml`, `requirements*.txt`;
- **git internals and configuration:** everything under `.git/` (including hooks, which would run on the developer's next commit), `.gitattributes` (which can name filter programs) and `.gitmodules`;
- **IDE configuration that can run tasks or builders:** `.vscode/`, `.idea/`, and Eclipse's `.project`, `.classpath` and `.settings/`;
- **tools that run on entering a folder or committing:** `.envrc`, `.pre-commit-config.yaml`.

Each repository can add to the list in a .agentignore file the agent also cannot edit.

**M8. Dependencies are suggested, not added.** When a change needs a new dependency, the agent discusses it (package, version, purpose) and the developer chooses to add it by hand or not add it. The model is requested (but cannot be forced) to present the dependency as a description, not a copy-and-paste-able import statement.

**M9. Logging.** Every agent action and refusal, and every plan approval, is logged with the user, session and time. Snapshots (M3) let the log separate the agent's changes from the developer's. Organization policy (M13) sets what is logged:
- **Full transcripts** (default): audit events plus the complete conversation, including code and test output the model saw.
- **Audit only:** actions, refusals, approvals, file paths and content hashes, without conversation text or file contents.

Logs are written locally first, to a file only the developer's account can read, and can be uploaded to central storage (M15).

**M10. Competitive on the core loop.** For plan → diffs → tests → MR → revise, the tool is capable enough that developers don't want to switch to an unrestricted product.

**M11. Terminal interface.** The conversation happens in a terminal, so it works alongside any IDE. No IDE plugin is required.

**M12. Files the agent cannot read.** Anything the agent reads is sent to the model API, so the harness refuses to read files that must never leave the developer's machine. The default list covers `.env*`, private keys and certificates (`*.pem`, `*.key`, `id_*`), and credential files. Each repository can add to it, for example data directories that hold restricted or regulated data, in `.agentignore` (M7). Search results never include these files' contents.

**M13. Organization policy.** The organization delivers a policy file through its usual software and configuration distribution. Developers cannot change it, and it overrides developer and repository settings. Repository configuration (such as `.agentignore`) can only add restrictions, never remove them. The policy covers at least:

| Setting | Default | Notes |
|---|---|---|
| **Log retention** | **5 years** | **Review this before the first production upload.** S3 Object Lock in compliance mode cannot shorten retention for logs already written (M15). |
| Log content | Full transcripts | Or audit only (M9) |
| Log destination | Local only | Or Amazon S3 (M15) |
| Model provider | None; must be set | Claude API, Amazon Bedrock or OpenAI-compatible (M16), plus the approved models |
| Minimum harness version | None | Older versions refuse to run (M17) |
| Sandboxed agent-run tests | Off | Only if C2 is built |
| Test service containers | None | Images the sandbox may start for tests, such as a database (C2) |
| Risky-code flags | Off | C3 |
| Protected and unreadable paths | The defaults in M7 and M12 | The organization can add to them |
| Source control host | None; must be set | Host URL and API type |

**M14. Reading test results.** The developer runs tests, in their IDE or terminal, as they normally would. The agent reads the results through a fixed, read-only action:
- **Standard reports.** The action reads JUnit XML reports, which most build tools write to a directory that is already gitignored (Maven `target/surefire-reports/`, Gradle `build/test-results/`, pytest `--junitxml`, JUnit reporters for Jest and Mocha). The results path is set in repository configuration the agent cannot edit, with defaults for common build tools.
- **Structured summary.** The agent receives which tests failed, assertion messages and stack traces, not raw console output.
- **Freshness.** The harness compares the report's time with the agent's last change (M3). If the developer hasn't rerun the tests since, it marks the results as stale.
- **Read-only.** The agent cannot write to the results path, so results in the log always come from a real run.
- **Output reaches the model.** Failure messages are sent to the model API, so tests must not print restricted data. M12 applies to the results path, and full-transcript logs (M9) record what the model saw.

IDE test runners (Eclipse's JUnit view, VS Code's Test Explorer) usually show results only in their own interface and don't write these reports. Developers need to run tests through the build tool, from the terminal or the IDE's build-tool integration, for the agent to see results.

Developers run agent-written tests before or after reviewing them, as they would a colleague's branch. Guidance to developers is to review agent changes before running them, as before committing them, not to avoid running tests.

**M15. Central log storage on Amazon S3 (organization option).** When organization policy selects S3:
- **Batched upload.** The harness appends events to the local log (M9) and uploads compressed chunks every few minutes and at session end. Failed uploads stay queued locally and are retried, including at the next start.
- **Write-only, short-lived credentials.** Developers sign in through IAM Identity Center. The role can only call `s3:PutObject` under that developer's own prefix. It has no read, list or delete permission, and there are no long-lived access keys. The harness holds the credentials, and the agent cannot use them because it has no shell.
- **Tamper evidence.** Each event includes a hash of the previous one, so gaps and edits are detectable. The bucket uses versioning and S3 Object Lock in compliance mode, so no one, including administrators, can delete or overwrite logs during the retention period set in M13.
- **Encryption and access.** SSE-KMS with a dedicated key. The bucket policy rejects non-TLS requests and unencrypted uploads, and Block Public Access is on. Full transcripts go under a separate prefix with their own KMS key and tighter decryption access than audit events.
- **Querying.** Logs are partitioned by date for querying with Athena, which costs nothing until someone runs a query.
- **Optional upgrades.** Kinesis Data Firehose in front of S3 for near-real-time delivery or very large volumes, and CloudWatch Logs for alerting (for example, on repeated protected-path refusals).

**M16. Model providers.** The harness owns the agent loop and every tool. It talks to models through a small provider interface, whose adapters only translate messages, tool definitions, tool calls and tool results. Every rule in this document is enforced in the harness, so it applies the same way whichever model is used. Organization policy selects a provider and lists the approved models.
- **Provider-specific features** such as prompt caching or extended reasoning are used when a provider offers them, never required.
- **Each model is evaluated before approval.** Models differ in how reliably they call tools, follow a plan and keep phases small. The project ships an evaluation suite that organizations run before adding a model to the approved list.
- **Each provider needs its own data-processing approval** (P4) and its own credential, held by the harness (M2).

Supported providers:
- **Claude API (Must).** A key per user or team, stored in the operating system's credential store (macOS Keychain, Windows Credential Manager, libsecret on Linux), never in an environment variable or a file in the repository. Console workspaces provide usage tracking, spending limits and key revocation.
- **Amazon Bedrock (Must).** Short-lived credentials from IAM Identity Center, for a role that can only call `bedrock:InvokeModel*` on the approved models. Usage falls under the organization's AWS agreement and is recorded in CloudTrail. When logs also go to S3 (M15), one sign-in can cover both, through a single role that can only invoke the approved models and write logs. Bedrock also hosts other vendors' models, which the same adapter can reach through Bedrock's Converse API.
- **OpenAI API and OpenAI-compatible endpoints (Should).** An API key stored as for the Claude API. The same adapter covers self-hosted open-weight models served behind an OpenAI-compatible endpoint (for example vLLM), for organizations that require inference to stay on their own infrastructure. Agentic coding is sensitive to model quality, so such models must pass the same evaluation.

Choosing a provider is configuration, not a separate implementation.

**M17. Distribution and managed configuration.** Organizations can deploy the tool and its policy (M13) with the device management they already use.
- **One executable, no runtime.** The harness is written in Go and built as a single executable for each operating system and processor type. The only other requirement on developer machines is `git`.
- **Standard packages.** A signed and notarized `.pkg` for macOS (Jamf, Intune, Kandji), a signed MSI for Windows (Intune, Configuration Manager), and `.deb` and `.rpm` packages for Linux (package repository, Ansible, Puppet). Homebrew and winget packages for individuals evaluating it.
- **Policy in a fixed system location.** macOS: a managed preferences domain delivered as an MDM configuration profile, or `/Library/Application Support/Hermit/policy.json`. Windows: `HKLM\SOFTWARE\Policies\Hermit` (Group Policy or Intune, with an ADMX template), or `C:\ProgramData\Hermit\policy.json`. Linux: `/etc/hermit/policy.json`.
- **Refuse a writable policy.** The harness will not start if the developer's account can modify the policy file, because then the developer could change it.
- **Unmanaged mode.** With no system policy, the harness runs on the developer's own settings and shows a clear "unmanaged" banner. The guarantees in this document hold only for managed installs.
- **Organization-controlled updates.** The harness never updates itself. Organizations push new versions through the same channel as installs, and the policy can set a minimum version below which the harness refuses to run.
- **Build from source.** Builds are reproducible from a documented command. Releases include checksums, an SBOM and signed build provenance, and the signing step is documented so organizations can sign their own builds.

### Should

**S1. Commands in prose.** The model is instructed (but not forced) to describe goals rather than write commands for the developer to run. If the model suggests shell commands, the harness flags them as unverified. 

**S2. Guided MR review** (task 6).

### Could

**C1. Repository orientation** (task 7).

**C2. Sandboxed tests the agent can run (organization option; decide after the pilot).** With M14, the developer runs every test. A sandbox would let the agent run tests, fix and rerun on its own, which is faster for "make these tests pass" tasks. Build it only if the pilot shows developers often want this (see section 9). If built:
- **Fixed command.** The test command is defined in repository configuration the agent cannot edit. The agent chooses which tests to run (a file or a test name) but never writes the command, so the harness can describe each run exactly.
- **Container sandbox.** Tests run in a container (Podman or Docker, which becomes a requirement on developer machines) with the repository mounted, no network access, no credentials in the environment, and writes limited to the repository and temporary space.
- **Service containers.** Organization policy can allow named service images, such as a database, on an isolated network with no outside access, holding synthetic or seeded data, never production data.
- **No approval for each run,** because the command is fixed and the sandbox limits what it can do.
- **Dependencies are fetched outside the sandbox** by the developer's normal build.
- **The developer can use it too,** to run agent-written tests contained before reviewing them.

**This is a real compromise.** Many non-trivial tests need real databases or services. Service containers cover some of this, but tests that need shared infrastructure, real data or outside services run only on the developer's machine or in CI.

**C3. Flags for risky code (organization option, off by default).** After each agent step, deterministic rules (not the model) point out lines in the agent's new code that deserve a closer look before running, such as network calls, reading credential paths, or starting processes. The risk is flag fatigue: warnings people learn to skip are worse than none. So rules are few and high-signal, apply only to code the agent added, and are tuned with pilot data before being turned on widely.

## 8. Organizational prerequisites

These aren't features of the tool, but the design depends on them:

- **P1.** In every repository the agent works on, CI secrets are available only to protected branches (in GitLab, CI/CD variables marked *protected*; other hosts have equivalents). Agent task branches are never protected.
- **P2.** Pipelines for agent branches run on runners with restricted network access and no deployment credentials. Agent-written tests run in CI, and test setup code can execute anything the runner allows. This is why P1 and P2 are essential even with M7.
- **P3.** MRs containing agent-written changes go through the same human review as any other MR.
- **P4.** Each model provider the organization enables is approved under its data-processing agreements.
- **P5.** If logs go to S3 or the model provider is Bedrock: an AWS account with IAM Identity Center, the log bucket and KMS keys set up as in M15, Bedrock model access enabled for the approved models, and a role limited to those permissions.
- **P6.** Repositories write JUnit XML test reports from their build tool (M14). If sandboxed agent-run tests (C2) are built and enabled: Podman or Docker on developer machines, and synthetic or seeded data for any service containers.

## 9. Success measures

To be agreed with leadership before a pilot:

- **Retention:** pilot users keep using the tool, and report (in an anonymous survey) how tempted they are to go around it.
- **Speed:** time from feature start to MR, compared with working without the tool.
- **Quality:** rework requests and defects found in MR review, compared with MRs written without the tool.
- **Understanding:** reviewers rate, in one quick step, whether the MR author could explain the change.
- **Friction:** how often protected-path or read refusals and dependency suggestions get in the way of legitimate work.
- **Demand for agent-run tests (decides C2):** how often developers wish the agent could run tests itself, and how many developer test runs a typical "fix the failing tests" task takes.

## 10. Purposefully out of scope

- The agent running commands, builds or tests. The developer runs tests and the agent reads the results (M14). Agent-run tests in a sandbox (C2) are a possible later addition.
- Changes to CI configuration, build scripts, dependencies, git internals or IDE configuration (see M7 and M8).
- Committing or pushing. The developer does both.
- Unattended or scheduled operation. A developer is present for every task.
- Access to any system other than the repository, the model API, the organization's source control host, and the logging solution.

## 11. Open questions

1. **Policy signing:** whether, beyond file permissions (M17), the harness should verify that the policy file is signed by the organization.
2. **Approved models:** which models to allow for each provider, what score on the evaluation suite (M16) a model needs, and whether the policy should allow a cheaper model for simple steps.
3. **Source control host credentials:** a per-user token (MRs attributed to the developer) or a project token; how the harness obtains and stores it.
4. **Phase size:** what counts as small enough to review and commit, and whether it is set per repository.
5. **Repositories without test reports or CI:** what the agent does when there are no JUnit reports to read, and whether it should help the developer set them up (the configuration files are protected, so the developer would make the change).
6. **Log volume:** expected size and cost of full-transcript logs over five years, and whether audit events need a longer retention than transcripts.
7. **Detecting commands in prose (S1):** how reliable detection can be, and what the developer sees.
8. **Eclipse users:** whether a terminal plus Eclipse's Git views is comfortable enough in practice.
9. **IDE test runners:** whether developers will run tests through the build tool so reports are written (M14), or whether the harness needs another way to collect IDE results.
10. **Containers on Windows and macOS (only if C2 is built):** performance of mounted repositories, and licensing (Docker Desktop requires a paid subscription for larger organizations; Podman does not).
