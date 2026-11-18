# Security policy

Hermit is a coding agent whose harness is the security boundary: the agent has no shell, no network access and no credentials, and every rule is enforced in the harness's code ([REQUIREMENTS.md](REQUIREMENTS.md), principle 6). A flaw in that enforcement is a security vulnerability, and we want to hear about it.

## Status

Hermit is in development and has no release yet. Until version 1.0, only the latest commit on `main` is supported, and nothing here is fit for production use. After 1.0, this section will list the supported versions.

## Reporting a vulnerability

Report vulnerabilities privately through GitHub: open the repository's **Security** tab and choose **Report a vulnerability**. Please don't open a public issue, pull request or discussion about a suspected vulnerability.

A useful report includes:
- what the flaw lets someone do, and which requirement it breaks (for example, "the agent can write to `.git/hooks/` despite M8");
- steps to reproduce, ideally as a scripted session for the scripted test provider (S4), which runs without a model;
- the operating system and file system, since several path rules differ between them (M21);
- the commit or version tested.

## What to expect

- **Acknowledgement** within 5 business days.
- **An initial assessment** within 14 days: whether we consider it a vulnerability, and how severe.
- **A fix**, or an agreed plan, within 90 days of the report for confirmed vulnerabilities. Severe flaws in the enforcement rules are fixed first.
- **Coordinated disclosure.** We publish a GitHub security advisory when a fix is available, and credit the reporter unless they prefer not to be named. If we can't meet these timelines, we'll say so and agree a new date with the reporter.

## Scope

In scope: anything that breaks a rule the harness enforces, for example:
- the agent running a command, opening a network connection, or obtaining a credential (M1, M2);
- reading a file the agent mustn't read, or learning its name (M13), or writing a protected, ignored or unreadable path (M8), including through symbolic links, hard links, case or Unicode tricks, or races (M21);
- content reaching the model without passing scanning, or a systematic way past a detector the organization has turned on (M19);
- text from the model or a repository that, when displayed, hides content, runs terminal escape sequences, or fetches a URL (M20);
- the source control client making a call outside its fixed list, or reaching another project (M24);
- a developer without administrator rights changing the policy or its effect, or altering logs without detection (M10, M16, M18);
- data sent to a provider, model, region or deployment the policy doesn't approve (M17, M23).

Out of scope: the threats [REQUIREMENTS.md](REQUIREMENTS.md) section 5 lists under "Outside this threat model", for example a developer with administrator rights, a developer who runs a modified build of Hermit, or a compromised model provider or source control host. Also out of scope:
- a model doing something unwise that the harness correctly contains. A prompt injection is a vulnerability only if it gets past the harness's enforcement;
- an individual piece of sensitive content that scanning misses, when no systematic bypass is involved. Scanning is documented as a backstop (M19);
- vulnerabilities in dependencies that Hermit doesn't reach. Please report those upstream, and tell us if Hermit is affected.

## Safe harbor

We won't pursue or support legal action against anyone who researches and reports in good faith under this policy: testing only on systems and data they're entitled to use, avoiding harm to others' data, and giving us reasonable time to fix a flaw before disclosing it.
