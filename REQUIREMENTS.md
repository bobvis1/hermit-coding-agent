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

If a developer uses a consumer product with no data processing agreement, it circumvents all of the organization's security requirements. Convenience, usability and usefulness to developers are thus security requirements. The tool should be competitive with tools a developer might encounter when working on less sensitive systems.

A developer determined to go around the tool can always find a way. The realistic aim is that going around it is unattractive, because the sanctioned tool is good, and visible, because server-side controls (the model gateway in M17 and the log checks in M16) record it.

## 3. Design principles

1. **Capabilities by construction.** The agent can do only what the harness offers as a named, structured action. There is no general shell to restrict, because there is no shell.
2. **The commit is the approval.** The agent reads, searches and edits files without asking, within the limits in M8 and M13. The developer approves the plan up front, then approves changes the way they already do: by reviewing and committing them. The agent never commits or pushes. The commit approves what is kept, not what runs: IDE tools can run agent edits before the developer reviews them (M22).
3. **Deterministic descriptions.** When the developer is asked to approve or review something, the harness's code describes it, not the model. The model's own explanation can be wrong or the product of a prompt injection. Where model-written text is itself the thing reviewed, such as an MR description that reviewers read as the author's explanation, the developer approves the exact text before it is sent (M20).
4. **Reversible by default.** The agent's changes stay uncommitted in the working tree until the developer commits them, and any agent step can be undone, including changes to files git does not track (M3). The agent's code reaches the source control host only when the developer pushes, and shared branches only through a merge request, which someone else reviews. Code that the developer's tools run locally can reach the network, so this holds for what the agent writes, not for what that code does if it runs (M22).
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
| Data exfiltration | Agent sends code or secrets somewhere | No network access for the agent; only outbound channels are the model API, the organization's source control host and its log storage, all used by the harness, not the agent; files that must never reach the model API cannot be read (M13). Note: logging is an exfiltration risk, but can be turned off |
| Leaks and deception through rendered text | An injected instruction gets the model to put an image in the MR description whose URL carries an encoded secret, fetched when a reviewer opens the MR; escape sequences or bidirectional characters hide part of a diff | External images removed and links shown in full; developer approves exact MR text; escape sequences stripped; hidden characters refused in writes and removed from reads (M20) |
| Sensitive content in ordinary files | A token in `config/local.yml`, personal data in a test fixture or saved notebook output, a secret in a test failure message | Git-ignored files unreadable by default (M13); scanning of everything sent to the model and everything written (M19); restricted data kept out of repositories (P7) |
| Local execution of agent-written files | Agent writes a git hook, IDE task or `.envrc` that runs on the developer's machine with their credentials | These paths are protected from edits by the model (M8), including everything under `.git/` |
| Bypassing path rules | A symlink in the repository to `~/.ssh/`, `.GIT/hooks/` on a case-insensitive file system, a hard link to a protected file, or a file swapped between check and write | Operations confined to the repository root and checked on resolved paths; permissive name matching; no symlink creation or multiply-linked files; atomic writes (M21); hostile tests on every operating system (S4) |
| Local execution of agent-written tests | A test or its setup code reads credentials or calls the network when the developer runs it | The agent's changes are shown separately for review (M3) before the developer runs them |
| Local execution the developer didn't choose | IDE test discovery imports an agent-written `conftest.py`, a linter loads an edited config file, build sync runs a build script, or a watch mode runs code on save | Guidance and recommended IDE settings; IDE configuration protected (M8); optional pending-changes mode holds edits outside the working tree until reviewed (M22) |
| Persistent injection into other tools | Agent edits `AGENTS.md`, `CLAUDE.md` or `.cursorrules` to plant instructions for a colleague's tool, or edits `CODEOWNERS` to change who must review | These files are protected (M8) |
| Reaching other systems | Agent queries a database or calls an internal service | No shell and no credentials, so no capability to do it |
| Side door through CI | Agent-written code reads CI secrets or calls the network when the pipeline runs | Protected paths; no protected variables or deployment credentials on agent branches; restricted runners |
| Supply chain | Agent adds a malicious or typo-squatted dependency | Agent cannot edit dependency manifests; it discusses trade-offs with the developer who decides and types |
| Subtle sabotage | An injected model weakens an authorization check or a cryptographic setting in a change that looks routine | Phase file lists show unplanned edits, and changes to sensitive areas are pointed out (M4); SAST and secret scanning in CI (P9); MR review (P3) |
| Hidden content in writes | Agent writes a binary file, a minified blob or a very long line that the IDE diff can't show meaningfully | Hard-to-review writes refused by default (M6) |
| Exposure through history or snapshots | A secret removed long ago is still in git history; snapshots stored as git refs are pushed with `git push --mirror` | No history access (M1); snapshots kept outside the repository and limited to files the agent changes (M3) |
| Human rubber-stamping | Developer commits everything unread near a deadline | A careless commit reaches only a branch and an MR that another human reviews, not the developer's machine or other systems |
| Data sent to the wrong provider or region | A restricted repository is worked on with a cloud model; a cross-region inference profile sends requests abroad; Bedrock invocation logging keeps copies of every prompt | Data classes by repository, with unknown repositories most restricted (M23); models approved as specific IDs or profiles (M17); invocation logging off or controlled (P5); provider terms confirmed before approval (P4) |
| Misuse of the source control token, and injection through the host | An injected instruction tries to read another project, merge or approve an MR, or delete a branch; an MR comment from an outside contributor carries instructions to the model; a CI summary leaks a masked secret | Fixed calls only, current project only, fixed branch rules, no merge, approval, label or comment actions (M24); MR diffs follow the repository's read rules; host content scanned and treated as untrusted (M19, M20, M24) |
| Credential reuse in other tools | Developer copies the model API key or Bedrock credentials from their machine and points an unrestricted agent at it | Organization model gateway: provider keys never reach laptops, short-lived SSO tokens, device certificates, request-shape checks, central logging and a kill switch (M17); without a gateway, cross-checks of provider records against Hermit's logs (M16) |
| Log tampering by the developer | Developer deletes the end of the local log before upload, or edits it | Hash chain; chain head anchored off the machine as the session runs; cross-checks against gateway or provider records (M16); Object Lock after upload |
| Policy bypass on the endpoint | Developer removes or corrupts the policy file to get unmanaged mode, or uses admin rights to change it | Fail closed on managed devices and on a broken policy (M18); developers are not local administrators (P8) |

There is no reliable mitigation to developers not reviewing code. Developers must take responsibility for the MRs they author and the MRs they approve.

### Outside this threat model

These are real threats, but this tool does not try to defend against them. Each needs other controls, which the organization should already have.

- **A developer who wants to leak data.** Someone determined can copy code by hand, photograph a screen or paste it into a consumer chatbot. The tool makes going around it visible (Goal 2), not impossible.
- **A developer with local admin rights working against the controls.** With admin rights, every local control can be undone (P8). Server-side controls make this visible but cannot prevent it.
- **A compromised developer machine.** Malware running as the developer can read anything the developer can, including credentials in the operating system's credential store. Endpoint protection, patching and disk encryption are the organization's responsibility.
- **A compromised model provider, gateway, source control host or CI system.** The tool trusts these services to do what they claim. Their security falls under the organization's vendor and infrastructure controls, and the provider's data handling under its agreements (P4).
- **Reviewers who approve without reading, or collude.** MR review (P3) is the last human check. If it fails, this tool cannot replace it.
- **A compromised Hermit release.** Signed, reproducible builds and organization signing (M18) reduce this risk; the organization's software intake process covers the rest.
- **Physical theft of a device.** Covered by the organization's device policies, such as disk encryption and remote wipe.
- **Persuasive but wrong explanations.** The model can explain wrong code convincingly. Small phases (M4), review in the IDE (M5) and MR review reduce this, but no tool can guarantee that a reviewer isn't persuaded.

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

**Terms.** The tool assumes Git for version control, because snapshots (M3) and commit-as-approval (M5) are built on it. It works with any **source control host** that offers merge requests and CI results through an API, such as GitLab, GitHub or Bitbucket, self-hosted or not. "MR" means a merge request, which some hosts call a pull request. **Organization policy** means settings the organization controls and developers cannot change (M14).

### Must

**M1. Capabilities.** The agent can only:
- read and search files in the repository, except the paths in M13;
- create, edit and delete files in the repository's working tree without asking, except the paths in M8;
- read MR comments, and test summaries and failure messages from CI pipelines (not full job logs, which often contain secrets), for the current project only (M24);
- write an MR description, and open or update an MR on the organization's source control host for a branch the developer has pushed, after the developer approves the exact text (M20), within the limits in M24;
- read the latest test results from the developer's test runs (M15).

The agent never commits or pushes. The developer does both.

**No git history.** The agent cannot read git history (no log, show or blame). History reopens what the read rules close: secrets removed from the current version are still there, and renamed paths escape `.agentignore`. If history access is added later, the current read rules (M13), path rules (M21) and scanning (M19) must apply to every historical path and version.

**M2. No shell, network or credentials.** The agent has no shell, no general network access, and never sees a credential. The harness holds the source control host token, the model credential and the log upload credential, and uses each only for its fixed purpose.

**M3. Shared working tree.** The agent works in the developer's normal working tree, alongside their uncommitted changes, in both directions: the developer can start work and have the agent finish it, or add their own touches after the agent's work. The harness:
- snapshots the tree before each agent step, so the review shows only the agent's changes for that step and the audit log can always tell the developer's work from the agent's;
- refuses a patch to any file that changed after the agent last read it, and has the agent re-read it first, so a developer's newer edits are never overwritten.

**Where snapshots live.** Snapshots are kept in the harness's own encrypted local storage (M10), never as git objects or refs in the repository, so commands such as `git push --mirror` cannot carry them anywhere. Only files the agent is about to change are captured. The agent cannot change unreadable files (M13), so files such as `.env` are never captured.

**Undo.** A `hermit undo` command restores the state before any agent step or phase, including files the agent edited, deleted or created that git does not track and so cannot restore.

**M4. Plan first, small phases.** The developer approves a phased plan before any code changes. The agent keeps each phase small enough for the developer to review and commit on its own, and splits larger phases.
- **Each phase lists its files.** The approved plan names the files (or folders) each phase expects to touch. The harness compares the agent's actual edits with that list and shows the difference as a fact, not the model's claim: "phase 2 planned `a.py` and `b.py`, and also changed `c.py`." It warns rather than refuses, and the developer can accept a wider scope with one confirmation.
- **Sensitive areas are pointed out.** Organization policy can list sensitive paths, such as authentication, cryptography and access control code. The phase summary notes when the agent changed one. The list is empty by default and should stay short, so the note appears rarely and only where the organization decided it matters.
- **Guidance, not a hard limit.** The model is instructed to aim for phases that can be reviewed by a human in a maximum of 1 hour. The harness reports each phase's size and warns, but does not refuse, when a phase exceeds a threshold set in organization policy. A hard limit would block legitimate changes such as renames and generated code. Starting values are tuned during the pilot. Line count is not a reliable measure of MR comprehensibility.

**M5. Review in the IDE; the developer commits.** The agent's changes appear in the working tree, so developers review them with their IDE's own diff and source-control views and can edit them directly. Committing is the approval. The harness never commits.

**M6. Writes must be reviewable.** IDE diffs cannot meaningfully show some content, so the harness refuses, by default, writes that contain:
- binary data;
- very long lines (over 500 characters by default);
- long base64-like or minified runs of text, detected by length and randomness.

Organization or repository configuration can allow named paths, such as test fixtures, where these are expected.

**M7. Tests.** The agent can write and update tests. The developer runs them locally, and CI runs them on push. The agent reads the results (M15 and M1) but does not run tests itself.

**M8. Protected paths.** The agent cannot create or change files that are executed or interpreted automatically, outside a reviewed code path. The harness refuses such changes before anything is written. The default protected list covers:
- **CI configuration:** for example `.gitlab-ci.yml`, `.github/workflows/`, `Jenkinsfile`, and files they include;
- **build scripts and dependency manifests:** `Makefile`, `Dockerfile`, `pom.xml`, `.mvn/`, `mvnw`, `build.gradle(.kts)`, `settings.gradle(.kts)`, `buildSrc/`, `gradle/wrapper/` (its download URL fetches and runs code), `package.json`, lockfiles, `pyproject.toml`, `setup.py`, `setup.cfg`, `requirements*.txt`, `Cargo.toml`, `build.rs`;
- **package manager and tool settings:** `.npmrc`, `.yarnrc.yml`, `.pypirc`, `pip.conf`, `.cargo/config.toml`;
- **containers:** `.devcontainer/`, `docker-compose*.yml`, `compose*.yml`;
- **git internals and configuration:** everything under `.git/` (including hooks, which would run on the developer's next commit), `.gitattributes` (which can name filter programs) and `.gitmodules`;
- **IDE configuration that can run tasks, builders or launches:** `.vscode/`, `*.code-workspace`, `.idea/`, and Eclipse's `.project`, `.classpath`, `.settings/`, `.externalToolBuilders/` and `*.launch` files;
- **tools that run on entering a folder or committing:** `.envrc`, `.pre-commit-config.yaml`, `.husky/`, `lefthook.yml`;
- **review rules:** `CODEOWNERS` (in any of its standard locations), which decides who must approve an MR (P3);
- **instructions for other AI tools:** `AGENTS.md`, `CLAUDE.md`, `.cursorrules`, `.github/copilot-instructions.md` and similar. Editing them would plant instructions for the next tool a colleague uses, a prompt injection that outlasts the session.

**Lists as data.** The defaults are kept as data files, one per ecosystem (for example Python, Java, JavaScript, Rust, containers, IDEs, AI tools), versioned and shipped with each release, so they can be updated without code changes. Organization policy can add to them.

**Repository additions.** Each repository can add to the list in a .agentignore file the agent also cannot edit.

**Optional write allowlist.** A repository can also list the only places the agent may write (for example `src/**` and `tests/**`). The protected list still applies inside those places. Because this only adds restrictions, repositories can set it themselves, and organization policy can require one.

**Scripts CI calls.** Scripts that CI configuration runs (for example `scripts/ci/*.sh`) are effectively CI configuration. The harness reads known CI configuration formats to find the scripts they reference and flags, but does not refuse, edits to them in the change summary. This is best effort, because CI configuration can reference scripts indirectly.

**M9. Dependencies are suggested, not added.** When a change needs a new dependency, the agent discusses it (package, version, purpose) and the developer chooses to add it by hand or not add it. The model is requested to present the dependency as a description, not a copy-and-paste-able import statement.

**M10. Logging.** Every agent action and refusal, and every plan approval, is logged with the user, session and time. Snapshots (M3) let the log separate the agent's changes from the developer's. Organization policy (M14) sets what is logged:
- **Audit only** (default): actions, refusals, approvals, file paths and keyed content hashes, without conversation text or file contents.
- **Audit plus transcripts** (organization opt-in): audit events plus the complete conversation, including code and test output the model saw. Transcripts can hold personal data or a secret that scanning (M19) missed, so they get shorter retention and can be erased (M16). Content scanning runs on transcripts before they are written.
- **None**

**Keyed hashes.** A plain hash of a short file or config value can be reversed by guessing. Each session gets a random hashing key, used for keyed hashes (HMAC). The key is stored with the log, encrypted with an organization KMS key, so only people allowed to decrypt with that key can verify hashes, and nobody else can brute-force them.

**The destination is a required choice.** Organization policy must say where logs go: Amazon S3 (M16), the model gateway's log (M17), or local only. A local-only log is controlled by the developer, so it is not an audit trail anyone can rely on, and the harness shows a warning when it is used.

Logs are written locally first and then uploaded (M16). A local log alone can be edited or cut short by the developer; anchoring and server-side cross-checks (M16) make that detectable.

**Files on the developer's machine.** The local log, conversation state, snapshots (M3) and pending changes (M22) all hold code and possibly data:
- **Encrypted at rest** with a key held in the operating system's credential store, and readable only by the developer's account. Full-disk encryption is also assumed (P8).
- **Local logs are deleted** once their upload is confirmed and a retention period set by organization policy has passed (a few days by default). If uploads fail, they are kept until uploaded.
- **Session files are cleaned up** at the end of a session or after a maximum age set by organization policy, whichever comes first.
- **Saved conversations are transcripts.** Conversation state kept so a session can be resumed is a transcript on the developer's machine, even when central transcripts are off. It is encrypted like other local files, and a session can be resumed only within the maximum age.
- **Uninstalling removes everything** the harness stored.

**One repository per session.** A session works in exactly one repository. Data classes (M23), read rules and source control limits (M24) all depend on knowing which repository is in use.

**M11. Competitive on the core loop.** For plan → diffs → tests → MR → revise, the tool is capable enough that developers don't want to switch to an unrestricted product.

**M12. Terminal interface.** The conversation happens in a terminal, so it works alongside any IDE. No IDE plugin is required. Developers can run it in the IDE's own terminal (Eclipse's Terminal view, VS Code's integrated terminal) to keep everything in one window.

**M13. Files the agent cannot read.** Anything the agent reads is sent to the model API, so the harness refuses to read files that must never leave the developer's machine. The default list covers `.env*`, private keys and certificates (`*.pem`, `*.key`, `id_*`), and credential files. Each repository can add to it, for example data directories that hold restricted or regulated data, in `.agentignore` (M8). Search results never include these files' contents.
- **Files git ignores are unreadable by default.** Ignored files are mostly local configuration, data dumps and build output, where sensitive content collects. The test results path (M15) is the exception. Organization policy, and only organization policy, can allow named ignored folders, such as dependency directories (`node_modules/`, `.venv/`, `vendor/`) whose library source helps the agent. Repositories cannot, because this loosens a restriction.
- **Names are hidden too.** A file name can itself be data, such as `data/patients/<name>.csv`. Unreadable files don't appear in listings, search results or the change summaries the model sees. A request for one by name gets a generic "not available" that doesn't confirm whether it exists. The developer's audit log (M10) still records the refusal.

Reading by file name is coarse: sensitive content also turns up in test fixtures, notebooks with saved output, local config files and test failures. Content scanning (M19) covers what file rules miss.

**M14. Organization policy.** The organization delivers a policy file through its usual software and configuration distribution. Developers cannot change it, and it overrides developer and repository settings. Repository configuration (such as `.agentignore`) can only add restrictions, never remove them.

**The policy contains no secrets.** It is on every machine and readable by every developer. It says how each developer obtains their own credentials, and the harness stores those in the operating system's credential store (M2). Per-developer credentials attribute MRs and usage to the right person and can be revoked one at a time. Section 13 lists everything an organization, a repository and a developer provide.

The policy covers at least:

| Setting | Default | Notes |
|---|---|---|
| **Log retention: audit events** | **5 years** | **Review this before the first production upload.** Audit events use S3 Object Lock in compliance mode, which cannot shorten retention for logs already written (M16). |
| **Log retention: transcripts** | **90 days**, if transcripts are turned on | **Review this too.** Transcripts contain code and possibly data. They use governance mode and per-session keys, so administrators can erase them (M16). |
| Phase size warning | Set during the pilot | A size measure (changed lines and files) standing in for about one hour of review. The harness warns above it, never refuses (M4) |
| Log content | Audit only | Or audit plus transcripts, or none (M10) |
| Log destination | None; must be set | Amazon S3 (M16), the model gateway's log (M17), or local only, which is not a reliable audit trail (M10) |
| Local file retention | A few days after confirmed upload; session files cleaned up at session end | For local logs and session files on developer machines (M10) |
| Model provider | None; must be set | Claude API, Amazon Bedrock, OpenAI-compatible, or an organization gateway (M17), plus the approved models as specific IDs or inference profiles. A gateway is recommended for production |
| Data classes | One class for all repositories | Repository patterns mapped to classes, and the providers and models each class may use. Unmatched repositories get the most restrictive class (M23) |
| Managed-device marker | None | MDM marker that makes a missing policy stop the harness (M18). Organization builds have it built in |
| Minimum harness version | None | Older versions refuse to run (M18) |
| Protected and unreadable paths | The defaults in M8 and M13 | The organization can add to them |
| Readable git-ignored folders | None | Exceptions to M13's ignored-file rule, such as dependency folders. Organization only |
| Content scanning (M19) | Secrets on; redact reads, refuse writes and MR text | Can switch reads to block |
| Organization data patterns | None | Precise patterns for the organization's own identifiers (M19) |
| Personal-data detection | Off | Generic detectors, opt-in (M19) |
| Allowed image and link domains | The source control host only | For images and links in MR text and markdown files (M20) |
| Paths allowing hidden or bidirectional characters | None | For example locale or translation files (M20). Organization only, because this loosens a restriction |
| Require a write allowlist | Off | Every repository must define where the agent may write (M8) |
| Pending-changes mode | Off | Agent changes held outside the working tree until the developer applies them (M22) |
| Source control host | None; must be set | Host URL and API type, and how developers obtain a token. Use the narrowest token the host offers (M24) |
| Allowed MR target branches | The default branch | Other branches MRs may target (M24) |
| Network | System settings | HTTPS proxy and extra CA certificates, often needed for self-hosted hosts and inspecting proxies |
| Usage limits | None | Tokens or spend per developer per day, and per session |
| Enabled | On | Set to off to stop the harness everywhere the policy reaches (section 12) |
| Sensitive paths | None | Paths whose changes are pointed out in phase summaries (M4). Keep the list short |
| Allowed hard-to-review content | None | Paths where binary data, long lines or encoded blobs may be written (M6) |
| Credential storage | Operating system credential store | Or the kernel keyring on headless Linux, or sign in each session (M18) |
| Session maximum age | Set by the organization | After this, session files are removed and a session cannot be resumed (M10) |
| Support details | None | Help channel or contact, and acceptable-use policy link, shown at startup and in errors |

**M15. Reading test results.** The developer runs tests, in their IDE or terminal, as they normally would. The agent reads the results through a fixed, read-only action:
- **Standard reports.** The action reads JUnit XML reports, which most build tools write to a directory that is already gitignored (Maven `target/surefire-reports/`, Gradle `build/test-results/`, pytest `--junitxml`, JUnit reporters for Jest and Mocha). The results path is set in repository configuration the agent cannot edit, with defaults for common build tools.
- **Structured summary.** The agent receives which tests failed, assertion messages and stack traces, not raw console output.
- **Freshness.** The harness compares the report's time with the agent's last change (M3). If the developer hasn't rerun the tests since, it marks the results as stale.
- **Read-only.** The agent cannot write to the results path, so results in the log always come from a real run.
- **Output reaches the model.** Failure messages are sent to the model API, so tests must not print restricted data. M13 applies to the results path, and transcripts, if turned on (M10), record what the model saw.

IDE test runners (Eclipse's JUnit view, VS Code's Test Explorer) usually show results only in their own interface and don't write these reports. Developers need to run tests through the build tool, from the terminal or the IDE's build-tool integration, for the agent to see results. Running Maven or Gradle tests from the IDE (Eclipse's m2e and Buildship, VS Code's Java extensions) writes the reports, and Eclipse's JUnit view can export a test run to XML.

**Without reports.** When there are no reports to read, the agent still works: it reads CI results through the source control host (M1), and the developer can paste a failure into the conversation. If the agent judges that the repository should write test reports or have CI, it drafts the change as a suggestion, as for dependencies (M9), and the developer applies it, since those files are protected (M8).

Developers run agent-written tests before or after reviewing them, as they would a colleague's branch. Guidance to developers is to review agent changes before running them, as before committing them, not to avoid running tests.

**M16. Central log storage on Amazon S3 (organization option).** When organization policy selects S3:
- **Batched upload.** The harness appends events to the local log (M10) and uploads compressed chunks every few minutes and at session end. Failed uploads stay queued locally and are retried, including at the next start.
- **Write-only, short-lived credentials.** Developers sign in through IAM Identity Center. The role can only call `s3:PutObject` under that developer's own prefix. It has no read, list or delete permission, and there are no long-lived access keys. The harness holds the credentials, and the agent cannot use them because it has no shell.
- **Tamper evidence.** Each event includes a hash of the previous one, so gaps and edits are detectable. The bucket uses versioning and S3 Object Lock.
- **Lock modes differ by content.**
  - **Audit events** use compliance mode, so no one, including administrators, can delete or overwrite them during their retention period (M14). They hold no code or data, so nothing in them should ever need erasing.
  - **Transcripts** use governance mode. It still stops developers, but named administrators with explicit permission can delete a transcript to clean up a spill.
- **Transcripts can be erased.** Each session's transcript is encrypted with its own data key, and the keys are kept in separate storage that can be deleted. Deleting a session's key makes its transcript permanently unreadable, even while Object Lock keeps the encrypted object (crypto-shredding). This is a common way to combine immutable logs with erasure requests and spill cleanup. Whether it meets a particular legal erasure requirement is a question for the organization's privacy office.
- **Anchored chain head.** A hash chain cannot show that its end was cut off: a developer could delete the last part of the local log before it is uploaded. So the harness records the session ID, event number and current chain hash off the machine as it goes. With a gateway (M17), it sends them with every model request and the gateway records them. Without one, it uploads them to S3 as a small separate object about every minute. The uploaded log must reach at least the last anchored point, or the gap is reported.
- **Cross-checks against server-side records.** Scheduled checks compare Hermit's sessions with records the developer cannot alter: the gateway's request log, or without a gateway the provider's own records (CloudTrail for Bedrock, which records that calls happened without their content, or usage per API key in the provider's console). Bedrock model invocation logs are not used, because they copy content (P5). Model usage with no matching Hermit session, and sessions whose logs stop short of their anchor, raise an alert.
- **Encryption and access.** SSE-KMS with a dedicated key. The bucket policy rejects non-TLS requests and unencrypted uploads, and Block Public Access is on. Transcripts, if turned on, go under a separate prefix with their own KMS key and tighter decryption access than audit events.
- **Querying.** Logs are partitioned by date for querying with Athena, which costs nothing until someone runs a query.
- **Cost.** Storage cost is small. A rough estimate for 50 developers, a few sessions a day and a couple of megabytes per transcript is about 15 GB a year compressed, or under 100 GB over five years. Lifecycle rules can move older logs to a Glacier storage class, and Object Lock still applies there. The real decision is whether to keep transcripts at all, and for how long, which is why they are off by default and have their own shorter retention (M14).
- **Optional upgrades.** Kinesis Data Firehose in front of S3 for near-real-time delivery or very large volumes, and CloudWatch Logs for alerting (for example, on repeated protected-path refusals).

**M17. Model providers.** The harness owns the agent loop and every tool. It talks to models through a small provider interface, whose adapters only translate messages, tool definitions, tool calls and tool results. Every rule in this document is enforced in the harness, so it applies the same way whichever model is used. Organization policy selects a provider and lists the approved models.
- **Provider-specific features** such as prompt caching or extended reasoning are used when a provider offers them, never required.
- **Each model is evaluated before approval.** Models differ in how reliably they call tools, follow a plan and keep phases small. The project ships an evaluation suite that organizations run before adding a model to the approved list.
- **Data residency.** Approved models are listed as specific model IDs or inference profiles, and the harness refuses anything else. Bedrock cross-region inference profiles send requests to other regions, some within one geography and some much more widely, so they must be approved explicitly. Where other providers offer regional endpoints, the policy names the endpoint. AWS GovCloud endpoints are supported for Bedrock and the other AWS services the harness uses (M16).
- **Each provider needs its own data-processing approval** (P4) and its own credential, held by the harness (M2).

Supported providers:
- **Claude API (Must).** A key per user or team, stored in the operating system's credential store (macOS Keychain, Windows Credential Manager, libsecret on Linux), never in an environment variable or a file in the repository. Console workspaces provide usage tracking, spending limits and key revocation.
- **Amazon Bedrock (Must).** Short-lived credentials from IAM Identity Center, for a role that can only call `bedrock:InvokeModel*` on the approved models. Usage falls under the organization's AWS agreement and is recorded in CloudTrail. When logs also go to S3 (M16), one sign-in can cover both, through a single role that can only invoke the approved models and write logs. Bedrock also hosts other vendors' models, which the same adapter can reach through Bedrock's Converse API.
- **OpenAI API and OpenAI-compatible endpoints including local ones (Must).** An API key stored as for the Claude API. The same adapter covers self-hosted open-weight models served behind an OpenAI-compatible endpoint (for example vLLM), for organizations that require inference to stay on their own infrastructure. Agentic coding is sensitive to model quality, so such models must pass the same evaluation.
- **Organization model gateway (Must support; recommended for production).** An organization-run proxy between developers and the model provider, reached through the Anthropic-compatible or OpenAI-compatible adapter. Developers authenticate to it with SSO and receive short-lived tokens; the provider keys stay on the gateway and never reach a laptop. Because it sits on the server, it applies to every request whichever client sent it:
  - central logging, content scanning, usage limits, and a kill switch for a user, a team or everyone;
  - optional device certificates issued by MDM, so only managed machines can connect;
  - a check that each request declares Hermit's fixed set of tools and no others, such as a shell tool. This stops casually pointing another agent at the gateway, but is not proof, because a determined developer can imitate Hermit's requests;
  - recording the log chain head that the harness sends with each request (M16).

**Why the gateway matters.** A raw Claude or OpenAI key in the developer's credential store, or Bedrock role credentials, works in any tool. A developer can point an unrestricted agent at it, under the organization's own data agreement and invisible to the organization: exactly the circumvention Goal 2 worries about. Raw keys and direct Bedrock credentials remain acceptable for pilots and lower-risk settings, with this risk accepted. Without a gateway, provider-side records are cross-checked against Hermit's logs (M16).

Choosing a provider is configuration, not a separate implementation.

- **Local model servers.** The OpenAI-compatible adapter also works with model servers on the developer's own machine (for example Ollama, LM Studio, llama.cpp's server or vLLM), so nothing leaves the machine. This suits testing the harness and evaluating local models. Local models are usually weaker at multi-step tool use, so they must pass the same evaluation before an organization approves one for real work.
- **Encrypted connections except on loopback.** The harness requires HTTPS for every model endpoint, except loopback addresses (`localhost`, `127.0.0.1`, `::1`), where plain HTTP is allowed. A configuration mistake can't send code over an unencrypted network connection.

**M18. Distribution and managed configuration.** Organizations can deploy the tool and its policy (M14) with the device management they already use.
- **Supported environments.** Desktop macOS, Windows and Linux, headless Linux and remote development machines, and virtual desktops, including non-persistent ones.
- **Credential storage fallbacks.** The operating system's credential store is used where available. On headless Linux, where libsecret needs a desktop session, the kernel keyring is used. Organization policy can instead choose "sign in each session", which suits non-persistent virtual desktops, where stored credentials disappear at logout. SSO (the gateway or IAM Identity Center) makes signing in each session easy; API keys suit these environments poorly, which is another reason to prefer a gateway (M17).
- **One executable, no runtime.** The harness is written in Go and built as a single executable for each operating system and processor type. The only other requirement on developer machines is `git`.
- **Standard packages.** A signed and notarized `.pkg` for macOS (Jamf, Intune, Kandji), a signed MSI for Windows (Intune, Configuration Manager), and `.deb` and `.rpm` packages for Linux (package repository, Ansible, Puppet). Homebrew and winget packages for individuals evaluating it.
- **Policy in a fixed system location.** macOS: a managed preferences domain delivered as an MDM configuration profile, or `/Library/Application Support/Hermit/policy.json`. Windows: `HKLM\SOFTWARE\Policies\Hermit` (Group Policy or Intune, with an ADMX template), or `C:\ProgramData\Hermit\policy.json`. Linux: `/etc/hermit/policy.json`.
- **Refuse a writable policy.** The harness will not start if the developer's account can modify the policy file, because then the developer could change it. Signing the policy is not required in version 1: file permissions and managed delivery stop developers from changing it, and signing would only guard against administrators, who could replace the executable anyway. It remains a possible later hardening step.
- **Fail closed.** A policy that is present but unreadable, malformed or invalid always stops the harness; it never falls back to weaker settings. On a managed device, a missing policy also stops it. The harness treats a device as managed if either the executable is an organization build with "managed" set at build time, or an MDM marker exists (a managed-preferences domain on macOS, or a policy registry key on Windows, written only by device management).
- **Unmanaged mode.** Only when there is no policy and no sign that the device is managed does the harness run on the developer's own settings, with a clear "unmanaged" banner. The guarantees in this document hold only for managed installs.
- **Organization-controlled updates.** The harness never updates itself. Organizations push new versions through the same channel as installs, and the policy can set a minimum version below which the harness refuses to run.
- **Build from source.** Builds are reproducible from a documented command. Releases include checksums, an SBOM and signed build provenance, and the signing step is documented so organizations can sign their own builds.
- **FIPS build.** For settings that require FIPS 140-3 cryptography, a build uses Go's built-in FIPS mode (available since Go 1.24). Organizations check the current validation status of Go's cryptographic module against their own requirements.

**M19. Content scanning.** The harness scans everything sent to the model (file contents, search results, test and CI output, MR descriptions and comments, and developer pastes) and everything the agent produces (file writes and MR text).
- **Secrets: always on.** Known token formats, private key blocks, connection strings with passwords, and high-entropy values assigned to names like `password` or `token`. Detection of this kind is accurate enough not to cause alert fatigue.
- **Organization patterns: optional.** The organization supplies precise patterns for its own sensitive identifiers, such as record or study ID formats.
- **Personal-data detection: optional, off by default.** Generic detectors (for example national ID numbers, or card numbers that pass a checksum) are opt-in. Broad detection of names, dates and addresses is noisy in ordinary code and test fixtures, and constant false alarms would push developers to unrestricted tools.
- **What happens on a match.** Content going to the model is redacted by default (the match is replaced with a marker naming its type, so the agent can keep working), or blocked entirely if policy says so. A file write containing a match is refused. MR text containing a match is never sent to the host.
- **Developer pastes.** The harness can't stop a developer typing something, but it scans pastes and asks for confirmation before sending anything that matches.
- **Logged without the value.** Each match is logged (M10) with its type and location, never the matched text.

Scanning is a backstop, not a guarantee. It reduces the chance of sensitive content reaching the model or the host, but it can miss things. The primary control is keeping restricted data out of repositories (P7).

**M20. Safe rendering.** The agent has no network access, but text it writes is displayed by software that does: the source control host, reviewers' browsers and the developer's terminal. Display is an output channel, so the harness controls what the model's text can do when rendered.
- **Images and links in MR text.** A markdown image is fetched as soon as someone opens the MR, with no click, and its URL can carry data to an outside server. This works even through an image proxy such as GitHub's, because the proxy fetches the URL itself. Content scanning (M19) can't be relied on here, because the model can encode the data first. So images are removed from MR text unless they point to the organization's source control host, and links are shown as plain text with the full URL visible. Organization policy can allow more domains.
- **Markdown files the agent writes.** Source control hosts can render `.md` files in the MR's diff view, which fetches their images the same way. A write that adds an image from a domain that isn't allowed is refused.
- **The developer approves the exact MR text.** Before any MR description or update is sent, the harness shows the exact text, unrendered and with every URL visible, and sends it only after the developer approves. Reviewers read the description as the author's explanation, so the developer should stand behind it (principle 3).
- **Escape sequences are stripped.** Terminal escape sequences can rewrite the screen to hide part of a diff or plan, and OSC 8 hyperlinks can hide a URL behind innocent text. Everything the harness displays (the model's replies, file excerpts, diffs, test output and CI summaries) has escape sequences removed. Only the harness's own formatting is allowed, and hyperlinks are shown as plain text with the URL visible.
- **Invisible and bidirectional characters.** Unicode tag characters and bidirectional override and isolate controls can hide code from reviewers (as in the Trojan Source attack) or smuggle data and instructions.
  - **In writes,** these characters are refused in code by default. Other zero-width characters are flagged in the change summary. They have legitimate uses, such as right-to-left languages in translation files and zero-width joiners in emoji, so organization policy can allow them in named paths, such as locale files.
  - **In reads,** repository content can use them to hide instructions to the model that the developer can't see. Content sent to the model has tag characters removed, and the developer is told when a file contained hidden characters.

**M21. Path safety.** Protected and unreadable paths (M8, M13) are only as strong as the path matching behind them, and matching path strings is easy to get around. The harness:
- **Confines every file operation to the repository root** through a directory handle opened at the root, not through path strings. `..` and symlinks that lead outside the root are refused. (Go provides this as `os.Root`.)
- **Checks rules against the resolved path.** A symlink that stays inside the repository can be followed, but M8 and M13 apply to where it actually points, not the name it was reached by.
- **Never creates symlinks and never writes through one.** A write replaces the file itself, never a link's target.
- **Refuses multiply-linked files.** A hard link is another name for the same file, invisible to path checks. Files with more than one link cannot be written, and cannot be read outside ignored folders the organization has allowed (M13). (Some package managers, such as pnpm, fill `node_modules` with hard links; allowing that folder accepts this.)
- **Compares names as the most permissive file system would, on every platform.** Matching ignores case and Unicode normalization differences, so `.GIT/hooks` and `.VSCode/` are protected even on Linux. Paths containing `:` (Windows alternate data streams), trailing dots or spaces, 8.3 short names (such as `GIT~1`) or reserved device names (such as `CON` or `NUL`) are refused.
- **Protects `.git` in every form, at any depth.** A `.git` file, as used by worktrees and submodules, is protected like a `.git` folder, as is the `.git` of any nested repository. The git directory such a file points to is normally outside the root, so confinement already blocks it.
- **Leaves no gap between check and use.** Writes go to a temporary file in the same folder, opened without following links, and are then atomically renamed into place. Reads open the file first and check the open file, so it can't be swapped between the check and the read.

Every case above has a hostile script in the test suite (S4).

**M22. Code that runs without the developer deciding.** Development tools run code from the working tree on their own:
- test discovery that imports test files and setup code (for example VS Code's Python test discovery importing `conftest.py`);
- editor extensions that load JavaScript configuration such as `eslint.config.js` or `jest.config.js`;
- Gradle and Maven project sync, which runs build scripts;
- watch modes and hot-reload servers that run code on every save.

So an agent edit can run on the developer's machine, with their credentials and network access, before anyone has reviewed it. Protected paths (M8) cannot prevent this, because test and source files are exactly what the agent is meant to write. The risk comes mainly from a prompt-injected model, which M13, M19 and M20 make harder but not impossible. The harness offers three levels of protection:
- **Guidance (default).** Documentation and recommended IDE settings turn off automatic test discovery, build sync and watch modes in repositories where the agent works. Where an IDE supports centrally managed settings, organizations can enforce them.
- **Protected IDE configuration (default).** The agent cannot change IDE settings or launch files (M8), so it cannot turn these features back on.
- **Pending-changes mode (organization option, off by default).** The agent's changes for each phase are held outside the working tree, where the IDE cannot see them. The developer reviews the diff, as text in the terminal or as a patch opened in the IDE, and then applies it. Nothing the agent writes can run until a person has seen it. This costs convenience (no editing in place, and an extra step for every phase), so it is for organizations whose security review cannot accept the residual risk.

**M23. Data classes by repository.** Not every repository can use every provider. Organization policy defines data classes and maps repositories to them. Each class lists the providers and models it may use, for example "public: any approved model", "internal: the gateway", "restricted: the local model only, or no model at all". Export-control rules, such as ITAR, are usually expressed this way.
- **Repository identity.** The harness identifies a repository by its git remote URL, matched against patterns in the policy. A developer can change a remote in seconds, so a repository that matches no pattern gets the most restrictive class, never the least. Disguising a repository as a less restricted one is deliberate circumvention, which the logs (M10) and gateway records show.
- **Enforced at the gateway too.** With a gateway (M17), the harness sends the repository identity with each request, and the gateway applies the same mapping on the server.
- **Shown to the developer.** The harness shows the repository's class at startup, so a developer knows which models are in use and why.

**M24. Source control access.** The agent never sees the source control token (M2), but the harness holds it, and host tokens are coarse. A GitLab token with the `api` scope, for example, can read every project the developer can see, and can merge, approve and delete branches. So the harness limits itself:
- **Fixed calls only.** The harness's source control client implements a fixed list of calls: read the current project; list and read its MRs and their comments; read test reports and failure summaries from its pipelines; and create or update an MR's title and description after exact-text approval (M20). There is no general way to call the host's API, so a prompt injection has nothing to steer.
- **Current project only.** The project is identified from the repository's git remote, as in M23. Any call that would reach another project is refused, including following links in MR text.
- **Fixed branch rules.** The source branch is always the branch the developer pushed. The target branch is the default branch or one on a list in organization policy.
- **Never:** merging, approving, enabling auto-merge, assigning reviewers or assignees, changing labels, deleting branches or posting comments. These stay human actions.
- **Repository rules apply to MR diffs.** Parts of an MR diff that touch files unreadable under M13 or `.agentignore` are hidden from the model, since reading a diff is another way of reading those files. Content scanning (M19) and path rules apply to diffs as they do to files.
- **Host content is untrusted.** MR descriptions and comments, and CI output, can be written by anyone with access, including outside contributors on some projects, and CI masking of secrets is imperfect. They pass through content scanning (M19) and hidden-character removal (M20). The model is also told they are data from other people, not instructions, but that is only a request (principle 6); the real protection is that the harness offers nothing dangerous to steer.
- **Narrower tokens where the host allows them.** Organizations use the narrowest token their host offers, such as GitHub fine-grained tokens limited to specific repositories and permissions, or project-scoped tokens. Where only coarse tokens exist, the harness's own limits above are the control.

### Should

**S1. Commands in prose.** The model is instructed (but not forced) to describe goals rather than write commands for the developer to run. If the model suggests shell commands, the harness flags them as unverified. Detection covers fenced code blocks marked as shell and lines that start with a shell prompt such as `$`, which is reliable. The harness shows them with a label ("unverified command from the model: read before running") and no copy button. Commands buried in a sentence are harder to detect, which is why the model is also instructed not to write them.

**S2. Guided MR review** (task 6). Read-only, and limited to MRs in the current project (M24), so it never pulls in code from other projects and their own rules. The current project's rules (M13, M19) apply to the MR's diff. Comments on the MR are written by the human reviewer, not the agent.

**S3. Setup check.** A `hermit doctor` command confirms that the policy loads and the developer cannot modify it, that credentials work, that the model provider and source control host are reachable with the configured proxy and CA certificates, that the test results path is readable, and, when S3 logging is on, that an upload succeeds. Each failure names the setting to fix and the support contact.

**S4. Scripted test provider.** A fake model provider replays a fixed sequence of model responses and tool calls, so the harness's own test suite is deterministic, fast and free. Scripts include deliberately hostile sequences, such as writing `.git/hooks/pre-commit`, reading `.env`, overwriting a file the developer has just changed, or emitting malformed tool calls, to prove that every rule is enforced by the harness regardless of the model. They also cover every path-safety case in M21:
- a symlink in the repository to `~/.ssh/`, and one into `.git/hooks/`;
- `..` escapes;
- `.GIT/hooks/` and `.VSCode/tasks.json`;
- a protected name written with different Unicode normalization;
- `GIT~1`, `file:stream` and trailing-dot names on Windows;
- a `.git` file in a worktree, and a nested repository;
- a hard link to a protected or unreadable file;
- a file replaced with a symlink between the check and the write.

The suite runs on macOS, Windows and Linux, because several cases exist on only one of them.

The scripted provider is available only in test builds or unmanaged mode, never through organization policy. Testing then uses three layers: the scripted provider for automated tests, a local model for manual smoke tests, and real providers for the evaluation suite (M17).

**S5. Fuzzing.** The harness is the security boundary, so the parts that handle untrusted input are fuzzed continuously: the path layer (M21), content scanning (M19), escape and hidden-character handling (M20), the tool-call parser, and the JUnit XML reader (M15). Untrusted XML is parsed with size limits and without external entities.

## 8. Organizational prerequisites

These aren't features of the tool, but the design depends on them:

- **P1.** In every repository the agent works on, CI secrets are available only to protected branches (in GitLab, CI/CD variables marked *protected*; other hosts have equivalents). Agent task branches are never protected.
- **P2.** Pipelines for agent branches run on runners with restricted network access and no deployment credentials. Agent-written tests run in CI, and test setup code can execute anything the runner allows. This is why P1 and P2 are essential even with M8.
- **P3.** MRs containing agent-written changes go through the same human review as any other MR.
- **P4.** Each model provider the organization enables is approved under its data-processing agreements. The harness cannot verify how a provider handles data, so before approval the organization confirms, in the contract or the provider's terms:
  - whether requests are used for training;
  - how long requests and responses are kept, including retention for abuse monitoring, which can apply even when other retention is off;
  - whether zero data retention is available, and whether it is in place for this account;
  - the provider's subprocessors;
  - where requests are processed and stored.

  The policy can record which terms were approved, for the audit trail, but the guarantee comes from the contract.
- **P5.** If logs go to S3 or the model provider is Bedrock: an AWS account with IAM Identity Center, the log bucket and KMS keys set up as in M16, Bedrock model access enabled for the approved models, and a role limited to those permissions. **Bedrock model invocation logging**, if turned on in the account, copies every prompt and response to CloudWatch or S3, outside Hermit's retention, encryption and erasure rules. It is either off, or placed under the same controls as transcripts (M16). `hermit doctor` (S3) reports the setting when the role is allowed to read it.
- **P6.** Repositories write JUnit XML test reports from their build tool (M15).
- **P7.** Restricted and regulated data, and real credentials, are kept out of source repositories, including test fixtures and saved notebook output. File rules (M13) and content scanning (M19) are a backstop for mistakes, not a substitute.
- **P8.** Developers are not local administrators on the machines where the harness runs. The local controls (a policy the developer cannot change, protected files, the local log before upload) hold only for developers without admin rights. Where developers are administrators, those controls depend on the developer's good faith, and the server-side controls (the model gateway in M17 and the log checks in M16) are what remain.

- **P9.** CI runs static analysis (SAST) and secret scanning on every branch that agent-written code reaches. These are the deep checks for subtle bugs and planted weaknesses that review can miss.
- **P10.** Before production use, the harness gets an independent penetration test, repeated for major releases. Once the agent has no shell, the harness is the security boundary and needs the testing a boundary gets.

## 9. Success measures

To be agreed with leadership before a pilot:

- **Retention:** pilot users keep using the tool, and report (in an anonymous survey) how tempted they are to go around it.
- **Speed:** time from feature start to MR, compared with working without the tool.
- **Quality:** rework requests and defects found in MR review, compared with MRs written without the tool.
- **Understanding:** reviewers rate, in one quick step, whether the MR author could explain the change.
- **Friction:** how often protected-path or read refusals, scanning false positives and dependency suggestions get in the way of legitimate work.
- **Demand for agent-run tests:** how often developers wish the agent could run tests itself, and how many developer test runs a typical "fix the failing tests" task takes. This shows whether a sandbox for agent-run tests is worth considering later.

## 10. Purposefully out of scope

- The agent running commands, builds or tests. The developer runs tests and the agent reads the results (M15). Agent-run tests in a sandbox could be considered later if the pilot shows demand.
- Changes to CI configuration, build scripts, dependencies, git internals or IDE configuration (see M8 and M9).
- Committing or pushing. The developer does both.
- Unattended or scheduled operation. A developer is present for every task.
- Access to any system other than the repository, the model API, the organization's source control host, and the logging solution.

## 11. Open questions

### To answer in the pilot

1. **Eclipse users:** whether running the harness in Eclipse's Terminal view, alongside its Git views, is comfortable enough in practice (M12).
2. **IDE test runners:** how often developers run tests in ways that don't write reports, and whether pasting failures into the conversation is an acceptable fallback (M15).
3. **Phase size:** the starting warning threshold and model guidance (M4).

### For each organization to decide

4. **Approved models:** which models to allow for each provider, and what score on the evaluation suite (M17) a model needs. Version 1 uses one model per session; a cheaper model for simple steps can be added later if cost warrants it.

### Later hardening

5. **Policy signing:** verifying that the policy file is signed by the organization, beyond file permissions and managed delivery (M18).

## 12. Operations and incident response

### Kill switch

- **With a gateway:** turn access off for a user, a team or everyone at the gateway (M17). This takes effect on the next request.
- **Without a gateway:** revoke the affected API keys or the Bedrock role, and push a policy with `enabled` set to false through device management. The harness refuses to start, and running sessions stop at their next policy check.

### Runbook: restricted data was sent to the model

1. **Find the affected sessions** in the audit log (M10, M16) by user, repository, time and scanning or refusal events.
2. **Rotate** any credential that may have been exposed. Assume exposure if in doubt.
3. **Ask the provider to delete the data**, where its terms allow it (P4). Record the request and the response.
4. **Erase affected transcripts** by deleting their session keys (crypto-shredding, M16), and clear local session files on the developer's machine.
5. **Fix the cause:** add the path to the unreadable list (M13), add an organization data pattern (M19), or move the data out of the repository (P7).
6. **Notify** according to the organization's own incident and data-breach policies.

## 13. Configuration reference

What each party provides to get the tool running. Nothing in the organization or repository layers is a secret.

### Organization (policy file, M14)

**Required**

| Setting | Contents |
|---|---|
| Model provider | Claude API, Amazon Bedrock, OpenAI-compatible, or an organization gateway (recommended for production) (M17) |
| Approved models | Specific model IDs or inference profiles the harness may use. Cross-region profiles only if approved (M17) |
| Provider endpoint | Bedrock region (including GovCloud), or the base URL of an OpenAI-compatible, regional or gateway endpoint |
| Model credential method | Gateway: the SSO sign-in details for short-lived tokens, and optionally the device certificate to present. Claude API or OpenAI: developers enter a key issued to them. Bedrock: IAM Identity Center start URL, sign-in region, account ID and role name |
| Managed-device marker | How the harness recognizes a managed device, if not an organization build (M18) |
| Source control host | URL and type (GitLab, GitHub, Bitbucket) |
| Source control credential method | An OAuth application ID, where the host supports device sign-in. Otherwise a personal access token with the narrowest scope that can read the repository and open MRs |
| Log destination | S3 (bucket, region, upload role, the audit KMS key, and for transcripts their own KMS key and key store), the model gateway's log, or local only (M10, M16) |

**Often needed**

| Setting | Contents |
|---|---|
| Extra CA certificates | Internal certificate authorities used by self-hosted hosts or inspecting proxies |
| HTTPS proxy | When outbound traffic must go through one |

**Defaults to review**

| Setting | Default |
|---|---|
| **Log retention: audit events** | **5 years** |
| **Log retention: transcripts** | **90 days**, if turned on |
| Phase size warning | Set during the pilot, as a stand-in for about one hour of review |
| Log content | Audit only |
| Local file retention | A few days after confirmed upload |
| Data classes | One class for all repositories; unmatched repositories get the most restrictive class (M23) |
| Allowed MR target branches | The default branch (M24) |
| Extra protected and unreadable paths | None beyond M8 and M13 |
| Readable git-ignored folders | None |
| Content scanning | Secrets on; redact reads, refuse writes and MR text |
| Organization data patterns, personal-data detection | None, off |
| Allowed image and link domains | The source control host only |
| Paths allowing hidden or bidirectional characters | None |
| Require a write allowlist, pending-changes mode | Off, off |
| Usage limits | None |
| Support details | None |
| Sensitive paths, allowed hard-to-review content | None |
| Credential storage | Operating system credential store |
| Minimum harness version | None |

### Repository (protected files the agent cannot edit)

- `.agentignore`: extra protected and unreadable paths, and optionally a write allowlist (M8). These can only add restrictions.
- Paths where binary data, long lines or encoded blobs are expected, such as test fixtures (M6).
- Test results path, if it is not the build tool's default (M15).

### Developer (once, at first run)

- Sign in to the source control host.
- Sign in to the model gateway or through IAM Identity Center, or enter the model API key issued to them.

The harness stores these in the operating system's credential store, never in a file. `hermit doctor` (S3) checks the result.

### Example policy

Illustrative only; the final format is settled during implementation. This example uses Bedrock, a self-hosted GitLab with an internal CA, and S3 logging.

```json
{
  "minimum_version": "1.0.0",
  "model": {
    "provider": "bedrock",
    "region": "us-east-1",
    "approved_models": ["<bedrock-model-id-or-in-region-inference-profile-id>"],
    "allow_cross_region_profiles": false,
    "credentials": {
      "method": "identity_center",
      "start_url": "https://example.awsapps.com/start",
      "sso_region": "us-east-1",
      "account_id": "123456789012",
      "role_name": "HermitDeveloper"
    }
  },
  "data_classes": {
    "classes": {
      "internal": { "providers": ["bedrock"] },
      "restricted": { "providers": ["openai_compatible"], "endpoint": "http://localhost:11434/v1" }
    },
    "repositories": [
      { "remote": "gitlab.example.org/restricted/**", "class": "restricted" },
      { "remote": "gitlab.example.org/**", "class": "internal" }
    ],
    "unmatched": "restricted"
  },
  "source_control": {
    "type": "gitlab",
    "url": "https://gitlab.example.org",
    "credentials": { "method": "oauth_device", "client_id": "<oauth-application-id>" },
    "allowed_target_branches": ["main", "release/*"]
  },
  "network": {
    "https_proxy": "http://proxy.example.org:3128",
    "extra_ca_certificates": ["/etc/hermit/example-root-ca.pem"]
  },
  "logging": {
    "content": "audit_only",
    "retention": { "audit_events_years": 5, "transcripts_days": 90 },
    "local_retention_days_after_upload": 3,
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
  "enabled": true,
  "sensitive_paths": ["src/auth/**", "src/crypto/**"],
  "credential_storage": "os_store",
  "session_max_age_days": 7,
  "usage_limits": { "tokens_per_developer_per_day": 5000000 },
  "support": {
    "contact": "#hermit-help",
    "acceptable_use_url": "https://intranet.example.org/ai-acceptable-use"
  }
}
```
