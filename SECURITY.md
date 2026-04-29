# Security Policy

## Supported Versions

LFK is under active development. Security fixes are applied to the latest
release on the `main` branch. Older tagged releases are not maintained — please
upgrade to the latest version to receive security patches.

| Version          | Supported          |
| ---------------- | ------------------ |
| Latest release   | Yes                |
| Older releases   | No                 |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues,
discussions, or pull requests.**

Instead, report them privately using GitHub's private vulnerability reporting:

- Go to https://github.com/janosmiko/lfk/security/advisories/new
- Or navigate to the repository's **Security** tab → **Advisories** →
  **Report a vulnerability**

If you cannot use GitHub's private vulnerability reporting, you may instead
open a minimal public issue asking for a private contact channel — without
disclosing any vulnerability details — and a maintainer will follow up.

When reporting, please include as much of the following as possible:

- A description of the issue and its potential impact
- Steps to reproduce, or a proof-of-concept
- The affected version(s) of LFK
- Any suggested mitigations or fixes
- Whether you intend to disclose publicly, and your preferred timeline

## Response Process

You can expect the following from the maintainers:

- **Acknowledgement** within **14 days** of your report
- **Initial assessment** (severity, scope, reproducibility) within **30 days**
- **Fix or mitigation** for confirmed vulnerabilities as soon as practical,
  prioritized by severity
- **Coordinated disclosure**: we will work with you on a disclosure timeline
  and credit you in the release notes and advisory unless you prefer to remain
  anonymous

If you do not receive a response within 14 days, please escalate by opening a
minimal public issue requesting acknowledgement (without vulnerability
details).

## Scope

In scope:

- The LFK binary and its source code in this repository
- Dependencies vendored or pinned by this repository
- Build, release, and CI/CD configuration in `.github/workflows/`

Out of scope:

- Vulnerabilities in upstream Kubernetes, `kubectl`, or `client-go` — please
  report those to their respective projects
- Issues that require an already-compromised local machine, kubeconfig, or
  cluster credentials
- Social engineering of maintainers or users

## Safe Harbor

We support good-faith security research. If you make a good-faith effort to
comply with this policy during your research, we will:

- Consider your research to be authorized
- Work with you to understand and resolve the issue quickly
- Not pursue legal action related to your research

Thank you for helping keep LFK and its users safe.
