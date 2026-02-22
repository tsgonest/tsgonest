# Security Policy

## Supported versions

Only the latest release receives security fixes. We do not backport patches to
older versions.

| Version | Supported |
|---------|-----------|
| latest  | Yes       |
| older   | No        |

## Reporting a vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

### GitHub private disclosure (preferred)

Use GitHub's built-in private reporting:

1. Go to the [Security tab](https://github.com/tsgonest/tsgonest/security) of this repository.
2. Click **"Report a vulnerability"**.
3. Fill in the details and submit.

A maintainer will acknowledge the report within **48 hours** and aim to release
a fix within **14 days**, depending on severity and complexity. You will be
notified of progress throughout.

### What to include

A useful report includes:

- A clear description of the vulnerability and its potential impact.
- Steps to reproduce (a minimal reproduction is ideal).
- The version(s) affected.
- Any suggested fix or mitigation, if you have one.

## Scope

| In scope | Out of scope |
|----------|-------------|
| The `tsgonest` CLI binary | Third-party dependencies (report upstream) |
| `@tsgonest/runtime` npm package | The `typescript-go` submodule (report to Microsoft) |
| `@tsgonest/types` npm package | Vulnerabilities in test fixtures |
| The release / CI pipeline | |

## Disclosure policy

We follow [responsible disclosure](https://en.wikipedia.org/wiki/Responsible_disclosure).
We ask that you give us reasonable time to address the issue before any public
disclosure. We will credit reporters in the release notes unless you prefer to
remain anonymous.
