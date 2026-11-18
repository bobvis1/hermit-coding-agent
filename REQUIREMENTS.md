# Requirements: A Guarded Coding Agent

## 1. Problem

Organizations that handle sensitive data often accept a model API (their agreements allow the data processing) but not a general-purpose coding agent such as Claude Code. Their decision makers worry about:

- **Data exfiltration:** code or data leaving through a channel nobody reviewed.
- **Reach:** the agent touching systems it shouldn't.
- **Comprehension:** code being committed that developers do not understand.

Turning off command execution doesn't solve this. The agent then suggests commands for the developer to run, which are often chained pipelines that are hard to read. The developer becomes a checkpoint, which can get tired or less particular near deadlines. A developer copying a command out of a chat runs with the developer's full access, bypassing every control otherwise enforced on the model.

Manual approval has the same weakness. Safety ends up depending on the developer's patience and discipline balanced against their own deadlines and pressures.

## 2. Goals

> Every action that takes effect is one the developer understands and intends.

> The tool is sufficiently useful that developers don't circumvent it with more powerful, unsafe ones.

If a developer uses a consumer product with no data processing agreement, it circumvents all of the organizations security requirements. Convenience, usability and usefulness to developers are thus security requirements. The tool should be competitive with tools a developer might encounter when working on less sensitive systems.

## 3. Design principles

1. **Capabilities by construction.** The agent can do only what the harness offers as a named, structured action. There is no general shell to restrict, because there is no shell.
2. **The commit is the approval.** The agent reads, searches and edits files without asking, within the limits in M7 and M12. The developer approves the plan up front, then approves changes the way they already do: by reviewing and committing them. The agent never commits or pushes.
3. **Deterministic descriptions.** When the developer is asked to approve or review something, the harness's code describes it, not the model. The model's own explanation can be wrong or the product of a prompt injection. Where model-written text is itself the thing reviewed, such as an MR description that reviewers read as the author's explanation, the developer approves the exact text before it is sent (M19).
4. **Reversible by default.** The agent's changes stay uncommitted in the working tree until the developer commits them. Nothing leaves the developer's machine until the developer pushes, and code reaches shared branches only through a merge request, which someone else reviews.
5. **Understanding through normal review.** Comprehension comes from well-structured comprehensible diffs, not from quizzes or attestations that people would resent and skip.
6. **The harness enforces, the model can only be asked.** Instructions to the model are suggestions lost to context drift. Hard rules can only be enforced in harness code, not in careful prompting. Do not fully trust the model to understand developer intent.

## 4. Users

- Developers of all skill levels.
- Everyone, including experts, will at times accept suggestions without reading them. The design limits what such approvals can do. Users must be instructed to remain critical.
- Developers use different IDEs and operating systems.

## 5. Threat model

| Threat | Example | Main mitigations |
|---|---|---|
| Honest model mistake | Deletes the wrong code, misreads requirements | Plan approval, small phases reviewed before the developer commits, MR review |
| Prompt injection | Text in the repository or a dependency instructs the agent | No shell, no network, no credentials; fixed actions only; protected paths |
| Data exfiltration | Agent sends code or secrets somewhere | No network access for the agent; only outbound channels are the model API, the organization's source control host and its log storage, all used by the harness, not the agent; files that must never reach the model API cannot be read (M12). Note: logging is an exfiltration risk, but can be turned off |
| Leaks and deception through rendered text | An injected instruction gets the model to put an image in the MR description whose URL carries an encoded secret, fetched when a reviewer opens the MR; escape sequences or bidirectional characters hide part of a diff | External images removed and links shown in full; developer approves exact MR text; escape sequences stripped; hidden characters refused in writes and removed from reads (M19) |
| Sensitive content in ordinary files | A token in `config/local.yml`, personal data in a test fixture or saved notebook output, a secret in a test failure message | Git-ignored files unreadable by default (M12); scanning of everything sent to the model and everything written (M18); restricted data kept out of repositories (P7) |
| Local execution of agent-written files | Agent writes a git hook, IDE task or `.envrc` that runs on the developer's machine with their credentials | These paths are protected from edits by the model (M7), including everything under `.git/` |
| Local execution of agent-written tests | A test or its setup code reads credentials or calls the network when the developer runs it | The agent's changes are shown separately for review (M3) before the developer runs them |
| Reaching other systems | Agent queries a database or calls an internal service | No shell and no credentials, so no capability to do it |
| Side door through CI | Agent-written code reads CI secrets or calls the network when the pipeline runs | Protected paths; no protected variables or deployment credentials on agent branches; restricted runners |
| Supply chain | Agent adds a malicious or typo-squatted dependency | Agent cannot edit dependency manifests; it discusses trade-offs with the developer who decides and types |
| Human rubber-stamping | Developer commits everything unread near a deadline | A careless commit reaches only a branch and an MR that another human reviews, not the developer's machine or other systems |

There is no reliable mitigation to developers not reviewing code. Developers must take responsibility for the MRs they author and the MRs they approve.

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
- read MR comments, and test summaries and failure messages from CI pipelines (not full job logs, which often contain secrets);
- write an MR description, and open or update an MR on the organization's source control host for a branch the developer has pushed, after the developer approves the exact text (M19);
- read the latest test results from the developer's test runs (M14).

The agent never commits or pushes. The developer does both.

**M2. No shell, network or credentials.** The agent has no shell, no general network access, and never sees a credential. The harness holds the source control host token, the model credential and the log upload credential, and uses each only for its fixed purpose.

**M3. Shared working tree.** The agent works in the developer's normal working tree, alongside their uncommitted changes, in both directions: the developer can start work and have the agent finish it, or add their own touches after the agent's work. The harness:
- snapshots the tree before each agent step, so the review shows only the agent's changes for that step and the audit log can always tell the developer's work from the agent's;
- refuses a patch to any file that changed after the agent last read it, and has the agent re-read it first, so a developer's newer edits are never overwritten.

**M4. Plan first, small phases.** The developer approves a phased plan before any code changes. The agent keeps each phase small enough for the developer to review and commit on its own, and splits larger phases. The harness reports which plan step each set of changes implements.
- **Guidance, not a hard limit.** The model is instructed to aim for phases that can be reviewed by a human in a maximum of 1 hour. The harness reports each phase's size and warns, but does not refuse, when a phase exceeds a threshold set in organization policy. A hard limit would block legitimate changes such as renames and generated code. Starting values are tuned during the pilot. Line count is not a reliable measure of MR comprehensibility.

**M5. Review in the IDE; the developer commits.** The agent's changes appear in the working tree, so developers review them with their IDE's own diff and source-control views and can edit them directly. Committing is the approval. The harness never commits.

**M6. Tests.** The agent can write and update tests. The developer runs them locally, and CI runs them on push. The agent reads the results (M14 and M1) but does not run tests itself.

**M7. Protected paths.** The agent cannot create or change files that are executed or interpreted automatically, outside a reviewed code path. The harness refuses such changes before anything is written. The default protected list covers:
- **CI configuration:** for example `.gitlab-ci.yml`, `.github/workflows/`, `Jenkinsfile`, and files they include;
- **build scripts and dependency manifests:** `Makefile`, `Dockerfile`, `pom.xml`, `build.gradle`, `package.json`, lockfiles, `pyproject.toml`, `requirements*.txt`;
- **git internals and configuration:** everything under `.git/` (including hooks, which would run on the developer's next commit), `.gitattributes` (which can name filter programs) and `.gitmodules`;
- **IDE configuration that can run tasks or builders:** `.vscode/`, `.idea/`, and Eclipse's `.project`, `.classpath` and `.settings/`;
- **tools that run on entering a folder or committing:** `.envrc`, `.pre-commit-config.yaml`.

Each repository can add to the list in a .agentignore file the agent also cannot edit.

**M8. Dependencies are suggested, not added.** When a change needs a new dependency, the agent discusses it (package, version, purpose) and the developer chooses to add it by hand or not add it. The model is requested to present the dependency as a description, not a copy-and-paste-able import statement.

**M9. Logging.** Every agent action and refusal, and every plan approval, is logged with the user, session and time. Snapshots (M3) let the log separate the agent's changes from the developer's. Organization policy (M13) sets what is logged:
- **Full transcripts** (default): audit events plus the complete conversation, including code and test output the model saw.
- **Audit only:** actions, refusals, approvals, file paths and content hashes, without conversation text or file contents.
- **None**

Logs are written locally first, to a file only the developer's account can read, and can be uploaded to central storage (M15).

**M10. Competitive on the core loop.** For plan → diffs → tests → MR → revise, the tool is capable enough that developers don't want to switch to an unrestricted product.

**M11. Terminal interface.** The conversation happens in a terminal, so it works alongside any IDE. No IDE plugin is required. Developers can run it in the IDE's own terminal (Eclipse's Terminal view, VS Code's integrated terminal) to keep everything in one window.

**M12. Files the agent cannot read.** Anything the agent reads is sent to the model API, so the harness refuses to read files that must never leave the developer's machine. The default list covers `.env*`, private keys and certificates (`*.pem`, `*.key`, `id_*`), and credential files. Each repository can add to it, for example data directories that hold restricted or regulated data, in `.agentignore` (M7). Search results never include these files' contents.
- **Files git ignores are unreadable by default.** Ignored files are mostly local configuration, data dumps and build output, where sensitive content collects. The test results path (M14) is the exception. Organization policy, and only organization policy, can allow named ignored folders, such as dependency directories (`node_modules/`, `.venv/`, `vendor/`) whose library source helps the agent. Repositories cannot, because this loosens a restriction.
- **Names are hidden too.** A file name can itself be data, such as `data/patients/<name>.csv`. Unreadable files don't appear in listings, search results or the change summaries the model sees. A request for one by name gets a generic "not available" that doesn't confirm whether it exists. The developer's audit log (M9) still records the refusal.

Reading by file name is coarse: sensitive content also turns up in test fixtures, notebooks with saved output, local config files and test failures. Content scanning (M18) covers what file rules miss.

**M13. Organization policy.** The organization delivers a policy file through its usual software and configuration distribution. Developers cannot change it, and it overrides developer and repository settings. Repository configuration (such as `.agentignore`) can only add restrictions, never remove them.

**The policy contains no secrets.** It is on every machine and readable by every developer. It says how each developer obtains their own credentials, and the harness stores those in the operating system's credential store (M2). Per-developer credentials attribute MRs and usage to the right person and can be revoked one at a time. Section 12 lists everything an organization, a repository and a developer provide.

The policy covers at least:

| Setting | Default | Notes |
|---|---|---|
| **Log retention: audit events** | **5 years** | **Review this before the first production upload.** S3 Object Lock in compliance mode cannot shorten retention for logs already written (M15). |
| **Log retention: transcripts** | **5 years** | **Review this too.** Transcripts contain code and possibly data, so organizations may keep them for less time than audit events. Same Object Lock caution. |
| Phase size warning | Set during the pilot | A size measure (changed lines and files) standing in for about one hour of review. The harness warns above it, never refuses (M4) |
| Log content | Full transcripts | Or audit only (M9) |
| Log destination | Local only | Or Amazon S3 (M15) |
| Model provider | None; must be set | Claude API, Amazon Bedrock or OpenAI-compatible (M16), plus the approved models |
| Minimum harness version | None | Older versions refuse to run (M17) |
| Protected and unreadable paths | The defaults in M7 and M12 | The organization can add to them |
| Readable git-ignored folders | None | Exceptions to M12's ignored-file rule, such as dependency folders. Organization only |
| Content scanning (M18) | Secrets on; redact reads, refuse writes and MR text | Can switch reads to block |
| Organization data patterns | None | Precise patterns for the organization's own identifiers (M18) |
| Personal-data detection | Off | Generic detectors, opt-in (M18) |
| Allowed image and link domains | The source control host only | For images and links in MR text and markdown files (M19) |
| Paths allowing hidden or bidirectional characters | None | For example locale or translation files (M19). Organization only, because this loosens a restriction |
| Source control host | None; must be set | Host URL and API type, and how developers obtain a token |
| Network | System settings | HTTPS proxy and extra CA certificates, often needed for self-hosted hosts and inspecting proxies |
| Usage limits | None | Tokens or spend per developer per day, and per session |
| Support details | None | Help channel or contact, and acceptable-use policy link, shown at startup and in errors |

**M14. Reading test results.** The developer runs tests, in their IDE or terminal, as they normally would. The agent reads the results through a fixed, read-only action:
- **Standard reports.** The action reads JUnit XML reports, which most build tools write to a directory that is already gitignored (Maven `target/surefire-reports/`, Gradle `build/test-results/`, pytest `--junitxml`, JUnit reporters for Jest and Mocha). The results path is set in repository configuration the agent cannot edit, with defaults for common build tools.
- **Structured summary.** The agent receives which tests failed, assertion messages and stack traces, not raw console output.
- **Freshness.** The harness compares the report's time with the agent's last change (M3). If the developer hasn't rerun the tests since, it marks the results as stale.
- **Read-only.** The agent cannot write to the results path, so results in the log always come from a real run.
- **Output reaches the model.** Failure messages are sent to the model API, so tests must not print restricted data. M12 applies to the results path, and full-transcript logs (M9) record what the model saw.

IDE test runners (Eclipse's JUnit view, VS Code's Test Explorer) usually show results only in their own interface and don't write these reports. Developers need to run tests through the build tool, from the terminal or the IDE's build-tool integration, for the agent to see results. Running Maven or Gradle tests from the IDE (Eclipse's m2e and Buildship, VS Code's Java extensions) writes the reports, and Eclipse's JUnit view can export a test run to XML.

**Without reports.** When there are no reports to read, the agent still works: it reads CI results through the source control host (M1), and the developer can paste a failure into the conversation. If the agent judges that the repository should write test reports or have CI, it drafts the change as a suggestion, as for dependencies (M8), and the developer applies it, since those files are protected (M7).

Developers run agent-written tests before or after reviewing them, as they would a colleague's branch. Guidance to developers is to review agent changes before running them, as before committing them, not to avoid running tests.

**M15. Central log storage on Amazon S3 (organization option).** When organization policy selects S3:
- **Batched upload.** The harness appends events to the local log (M9) and uploads compressed chunks every few minutes and at session end. Failed uploads stay queued locally and are retried, including at the next start.
- **Write-only, short-lived credentials.** Developers sign in through IAM Identity Center. The role can only call `s3:PutObject` under that developer's own prefix. It has no read, list or delete permission, and there are no long-lived access keys. The harness holds the credentials, and the agent cannot use them because it has no shell.
- **Tamper evidence.** Each event includes a hash of the previous one, so gaps and edits are detectable. The bucket uses versioning and S3 Object Lock in compliance mode, so no one, including administrators, can delete or overwrite logs during the retention period set in M13.
- **Encryption and access.** SSE-KMS with a dedicated key. The bucket policy rejects non-TLS requests and unencrypted uploads, and Block Public Access is on. Full transcripts go under a separate prefix with their own KMS key and tighter decryption access than audit events.
- **Querying.** Logs are partitioned by date for querying with Athena, which costs nothing until someone runs a query.
- **Cost.** Storage cost is small. A rough estimate for 50 developers, a few sessions a day and a couple of megabytes per transcript is about 15 GB a year compressed, or under 100 GB over five years. Lifecycle rules can move older logs to a Glacier storage class, and Object Lock still applies there. The real decision is how long to keep sensitive transcripts, which is why audit events and transcripts have separate retention settings (M13).
- **Optional upgrades.** Kinesis Data Firehose in front of S3 for near-real-time delivery or very large volumes, and CloudWatch Logs for alerting (for example, on repeated protected-path refusals).

**M16. Model providers.** The harness owns the agent loop and every tool. It talks to models through a small provider interface, whose adapters only translate messages, tool definitions, tool calls and tool results. Every rule in this document is enforced in the harness, so it applies the same way whichever model is used. Organization policy selects a provider and lists the approved models.
- **Provider-specific features** such as prompt caching or extended reasoning are used when a provider offers them, never required.
- **Each model is evaluated before approval.** Models differ in how reliably they call tools, follow a plan and keep phases small. The project ships an evaluation suite that organizations run before adding a model to the approved list.
- **Each provider needs its own data-processing approval** (P4) and its own credential, held by the harness (M2).

Supported providers:
- **Claude API (Must).** A key per user or team, stored in the operating system's credential store (macOS Keychain, Windows Credential Manager, libsecret on Linux), never in an environment variable or a file in the repository. Console workspaces provide usage tracking, spending limits and key revocation.
- **Amazon Bedrock (Must).** Short-lived credentials from IAM Identity Center, for a role that can only call `bedrock:InvokeModel*` on the approved models. Usage falls under the organization's AWS agreement and is recorded in CloudTrail. When logs also go to S3 (M15), one sign-in can cover both, through a single role that can only invoke the approved models and write logs. Bedrock also hosts other vendors' models, which the same adapter can reach through Bedrock's Converse API.
- **OpenAI API and OpenAI-compatible endpoints including local ones (Must).** An API key stored as for the Claude API. The same adapter covers self-hosted open-weight models served behind an OpenAI-compatible endpoint (for example vLLM), for organizations that require inference to stay on their own infrastructure. Agentic coding is sensitive to model quality, so such models must pass the same evaluation.

Choosing a provider is configuration, not a separate implementation.

- **Local model servers.** The OpenAI-compatible adapter also works with model servers on the developer's own machine (for example Ollama, LM Studio, llama.cpp's server or vLLM), so nothing leaves the machine. This suits testing the harness and evaluating local models. Local models are usually weaker at multi-step tool use, so they must pass the same evaluation before an organization approves one for real work.
- **Encrypted connections except on loopback.** The harness requires HTTPS for every model endpoint, except loopback addresses (`localhost`, `127.0.0.1`, `::1`), where plain HTTP is allowed. A configuration mistake can't send code over an unencrypted network connection.

**M17. Distribution and managed configuration.** Organizations can deploy the tool and its policy (M13) with the device management they already use.
- **One executable, no runtime.** The harness is written in Go and built as a single executable for each operating system and processor type. The only other requirement on developer machines is `git`.
- **Standard packages.** A signed and notarized `.pkg` for macOS (Jamf, Intune, Kandji), a signed MSI for Windows (Intune, Configuration Manager), and `.deb` and `.rpm` packages for Linux (package repository, Ansible, Puppet). Homebrew and winget packages for individuals evaluating it.
- **Policy in a fixed system location.** macOS: a managed preferences domain delivered as an MDM configuration profile, or `/Library/Application Support/Hermit/policy.json`. Windows: `HKLM\SOFTWARE\Policies\Hermit` (Group Policy or Intune, with an ADMX template), or `C:\ProgramData\Hermit\policy.json`. Linux: `/etc/hermit/policy.json`.
- **Refuse a writable policy.** The harness will not start if the developer's account can modify the policy file, because then the developer could change it. Signing the policy is not required in version 1: file permissions and managed delivery stop developers from changing it, and signing would only guard against administrators, who could replace the executable anyway. It remains a possible later hardening step.
- **Unmanaged mode.** With no system policy, the harness runs on the developer's own settings and shows a clear "unmanaged" banner. The guarantees in this document hold only for managed installs.
- **Organization-controlled updates.** The harness never updates itself. Organizations push new versions through the same channel as installs, and the policy can set a minimum version below which the harness refuses to run.
- **Build from source.** Builds are reproducible from a documented command. Releases include checksums, an SBOM and signed build provenance, and the signing step is documented so organizations can sign their own builds.

**M18. Content scanning.** The harness scans everything sent to the model (file contents, search results, test and CI output, and developer pastes) and everything the agent produces (file writes and MR text).
- **Secrets: always on.** Known token formats, private key blocks, connection strings with passwords, and high-entropy values assigned to names like `password` or `token`. Detection of this kind is accurate enough not to cause alert fatigue.
- **Organization patterns: optional.** The organization supplies precise patterns for its own sensitive identifiers, such as record or study ID formats.
- **Personal-data detection: optional, off by default.** Generic detectors (for example national ID numbers, or card numbers that pass a checksum) are opt-in. Broad detection of names, dates and addresses is noisy in ordinary code and test fixtures, and constant false alarms would push developers to unrestricted tools.
- **What happens on a match.** Content going to the model is redacted by default (the match is replaced with a marker naming its type, so the agent can keep working), or blocked entirely if policy says so. A file write containing a match is refused. MR text containing a match is never sent to the host.
- **Developer pastes.** The harness can't stop a developer typing something, but it scans pastes and asks for confirmation before sending anything that matches.
- **Logged without the value.** Each match is logged (M9) with its type and location, never the matched text.

Scanning is a backstop, not a guarantee. It reduces the chance of sensitive content reaching the model or the host, but it can miss things. The primary control is keeping restricted data out of repositories (P7).

**M19. Safe rendering.** The agent has no network access, but text it writes is displayed by software that does: the source control host, reviewers' browsers and the developer's terminal. Display is an output channel, so the harness controls what the model's text can do when rendered.
- **Images and links in MR text.** A markdown image is fetched as soon as someone opens the MR, with no click, and its URL can carry data to an outside server. This works even through an image proxy such as GitHub's, because the proxy fetches the URL itself. Content scanning (M18) can't be relied on here, because the model can encode the data first. So images are removed from MR text unless they point to the organization's source control host, and links are shown as plain text with the full URL visible. Organization policy can allow more domains.
- **Markdown files the agent writes.** Source control hosts can render `.md` files in the MR's diff view, which fetches their images the same way. A write that adds an image from a domain that isn't allowed is refused.
- **The developer approves the exact MR text.** Before any MR description or update is sent, the harness shows the exact text, unrendered and with every URL visible, and sends it only after the developer approves. Reviewers read the description as the author's explanation, so the developer should stand behind it (principle 3).
- **Escape sequences are stripped.** Terminal escape sequences can rewrite the screen to hide part of a diff or plan, and OSC 8 hyperlinks can hide a URL behind innocent text. Everything the harness displays (the model's replies, file excerpts, diffs, test output and CI summaries) has escape sequences removed. Only the harness's own formatting is allowed, and hyperlinks are shown as plain text with the URL visible.
- **Invisible and bidirectional characters.** Unicode tag characters and bidirectional override and isolate controls can hide code from reviewers (as in the Trojan Source attack) or smuggle data and instructions.
  - **In writes,** these characters are refused in code by default. Other zero-width characters are flagged in the change summary. They have legitimate uses, such as right-to-left languages in translation files and zero-width joiners in emoji, so organization policy can allow them in named paths, such as locale files.
  - **In reads,** repository content can use them to hide instructions to the model that the developer can't see. Content sent to the model has tag characters removed, and the developer is told when a file contained hidden characters.

### Should

**S1. Commands in prose.** The model is instructed (but not forced) to describe goals rather than write commands for the developer to run. If the model suggests shell commands, the harness flags them as unverified. Detection covers fenced code blocks marked as shell and lines that start with a shell prompt such as `$`, which is reliable. The harness shows them with a label ("unverified command from the model: read before running") and no copy button. Commands buried in a sentence are harder to detect, which is why the model is also instructed not to write them.

**S2. Guided MR review** (task 6).

**S3. Setup check.** A `hermit doctor` command confirms that the policy loads and the developer cannot modify it, that credentials work, that the model provider and source control host are reachable with the configured proxy and CA certificates, that the test results path is readable, and, when S3 logging is on, that an upload succeeds. Each failure names the setting to fix and the support contact.

**S4. Scripted test provider.** A fake model provider replays a fixed sequence of model responses and tool calls, so the harness's own test suite is deterministic, fast and free. Scripts include deliberately hostile sequences, such as writing `.git/hooks/pre-commit`, reading `.env`, overwriting a file the developer has just changed, or emitting malformed tool calls, to prove that every rule is enforced by the harness regardless of the model. It is available only in test builds or unmanaged mode, never through organization policy. Testing then uses three layers: the scripted provider for automated tests, a local model for manual smoke tests, and real providers for the evaluation suite (M16).

## 8. Organizational prerequisites

These aren't features of the tool, but the design depends on them:

- **P1.** In every repository the agent works on, CI secrets are available only to protected branches (in GitLab, CI/CD variables marked *protected*; other hosts have equivalents). Agent task branches are never protected.
- **P2.** Pipelines for agent branches run on runners with restricted network access and no deployment credentials. Agent-written tests run in CI, and test setup code can execute anything the runner allows. This is why P1 and P2 are essential even with M7.
- **P3.** MRs containing agent-written changes go through the same human review as any other MR.
- **P4.** Each model provider the organization enables is approved under its data-processing agreements.
- **P5.** If logs go to S3 or the model provider is Bedrock: an AWS account with IAM Identity Center, the log bucket and KMS keys set up as in M15, Bedrock model access enabled for the approved models, and a role limited to those permissions.
- **P6.** Repositories write JUnit XML test reports from their build tool (M14).
- **P7.** Restricted and regulated data, and real credentials, are kept out of source repositories, including test fixtures and saved notebook output. File rules (M12) and content scanning (M18) are a backstop for mistakes, not a substitute.

## 9. Success measures

To be agreed with leadership before a pilot:

- **Retention:** pilot users keep using the tool, and report (in an anonymous survey) how tempted they are to go around it.
- **Speed:** time from feature start to MR, compared with working without the tool.
- **Quality:** rework requests and defects found in MR review, compared with MRs written without the tool.
- **Understanding:** reviewers rate, in one quick step, whether the MR author could explain the change.
- **Friction:** how often protected-path or read refusals, scanning false positives and dependency suggestions get in the way of legitimate work.
- **Demand for agent-run tests:** how often developers wish the agent could run tests itself, and how many developer test runs a typical "fix the failing tests" task takes. This shows whether a sandbox for agent-run tests is worth considering later.

## 10. Purposefully out of scope

- The agent running commands, builds or tests. The developer runs tests and the agent reads the results (M14). Agent-run tests in a sandbox could be considered later if the pilot shows demand.
- Changes to CI configuration, build scripts, dependencies, git internals or IDE configuration (see M7 and M8).
- Committing or pushing. The developer does both.
- Unattended or scheduled operation. A developer is present for every task.
- Access to any system other than the repository, the model API, the organization's source control host, and the logging solution.

## 11. Open questions

### To answer in the pilot

1. **Eclipse users:** whether running the harness in Eclipse's Terminal view, alongside its Git views, is comfortable enough in practice (M11).
2. **IDE test runners:** how often developers run tests in ways that don't write reports, and whether pasting failures into the conversation is an acceptable fallback (M14).
3. **Phase size:** the starting warning threshold and model guidance (M4).

### For each organization to decide

4. **Approved models:** which models to allow for each provider, and what score on the evaluation suite (M16) a model needs. Version 1 uses one model per session; a cheaper model for simple steps can be added later if cost warrants it.

### Later hardening

5. **Policy signing:** verifying that the policy file is signed by the organization, beyond file permissions and managed delivery (M17).

## 12. Configuration reference

What each party provides to get the tool running. Nothing in the organization or repository layers is a secret.

### Organization (policy file, M13)

**Required**

| Setting | Contents |
|---|---|
| Model provider | Claude API, Amazon Bedrock, or OpenAI-compatible (M16) |
| Approved models | Model IDs the harness may use |
| Provider endpoint | Bedrock region, or the base URL of an OpenAI-compatible or proxied endpoint |
| Model credential method | Claude API or OpenAI: developers enter a key issued to them. Bedrock: IAM Identity Center start URL, sign-in region, account ID and role name |
| Source control host | URL and type (GitLab, GitHub, Bitbucket) |
| Source control credential method | An OAuth application ID, where the host supports device sign-in. Otherwise a personal access token with the narrowest scope that can read the repository and open MRs |

**Often needed**

| Setting | Contents |
|---|---|
| Extra CA certificates | Internal certificate authorities used by self-hosted hosts or inspecting proxies |
| HTTPS proxy | When outbound traffic must go through one |

**Defaults to review**

| Setting | Default |
|---|---|
| **Log retention: audit events** | **5 years** |
| **Log retention: transcripts** | **5 years** |
| Phase size warning | Set during the pilot, as a stand-in for about one hour of review |
| Log content | Full transcripts |
| Log destination | Local. For S3: bucket, region, KMS key ARNs for audit events and for transcripts, and the upload role |
| Extra protected and unreadable paths | None beyond M7 and M12 |
| Readable git-ignored folders | None |
| Content scanning | Secrets on; redact reads, refuse writes and MR text |
| Organization data patterns, personal-data detection | None, off |
| Allowed image and link domains | The source control host only |
| Paths allowing hidden or bidirectional characters | None |
| Usage limits | None |
| Support details | None |
| Minimum harness version | None |

### Repository (protected files the agent cannot edit)

- `.agentignore`: extra protected and unreadable paths. These can only add restrictions.
- Test results path, if it is not the build tool's default (M14).

### Developer (once, at first run)

- Sign in to the source control host.
- Sign in through IAM Identity Center, or enter the model API key issued to them.

The harness stores these in the operating system's credential store, never in a file. `hermit doctor` (S3) checks the result.

### Example policy

Illustrative only; the final format is settled during implementation. This example uses Bedrock, a self-hosted GitLab with an internal CA, and S3 logging.

```json
{
  "minimum_version": "1.0.0",
  "model": {
    "provider": "bedrock",
    "region": "us-east-1",
    "approved_models": ["<bedrock-model-or-inference-profile-id>"],
    "credentials": {
      "method": "identity_center",
      "start_url": "https://example.awsapps.com/start",
      "sso_region": "us-east-1",
      "account_id": "123456789012",
      "role_name": "HermitDeveloper"
    }
  },
  "source_control": {
    "type": "gitlab",
    "url": "https://gitlab.example.org",
    "credentials": { "method": "oauth_device", "client_id": "<oauth-application-id>" }
  },
  "network": {
    "https_proxy": "http://proxy.example.org:3128",
    "extra_ca_certificates": ["/etc/hermit/example-root-ca.pem"]
  },
  "logging": {
    "retention_years": { "audit_events": 5, "transcripts": 5 },
    "content": "full_transcripts",
    "destination": "s3",
    "s3": {
      "bucket": "example-hermit-logs",
      "region": "us-east-1",
      "audit_kms_key_arn": "arn:aws:kms:us-east-1:123456789012:key/<audit-key-id>",
      "transcript_kms_key_arn": "arn:aws:kms:us-east-1:123456789012:key/<transcript-key-id>"
    }
  },
  "paths": {
    "protected": ["deploy/**"],
    "unreadable": ["data/restricted/**"],
    "readable_ignored": ["node_modules/**", ".venv/**"]
  },
  "scanning": {
    "secrets": { "on_read": "redact" },
    "organization_patterns": [{ "name": "study-id", "regex": "STUDY-[0-9]{6}" }],
    "personal_data": false
  },
  "rendering": {
    "allowed_domains": ["gitlab.example.org"],
    "allow_hidden_characters_in": ["**/locales/**", "**/*.po"]
  },
  "usage_limits": { "tokens_per_developer_per_day": 5000000 },
  "support": {
    "contact": "#hermit-help",
    "acceptable_use_url": "https://intranet.example.org/ai-acceptable-use"
  }
}
```
