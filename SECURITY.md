# Security Policy

This project runs untrusted code, so security reports are taken seriously.

## Supported versions

The project has not had a stable release yet. Only the latest commit on `main` is supported.

## Reporting a vulnerability

**Do not open a public issue, discussion, or pull request for a vulnerability.**

Report it privately through GitHub: go to the repository's **Security** tab, choose **Report a
vulnerability**, and include:

- The affected component (`sandbox`, `scheduler`, `grader`, `frontend`, or `infra`) and the commit.
- Steps to reproduce. For a sandbox issue, include the submitted code.
- The impact: what an attacker gains.
- The host details if relevant: kernel version (`uname -r`), distribution, and cgroup mode.

The team will acknowledge the report, work with you on a fix, and credit you in the release notes
unless you prefer to stay anonymous.

## Scope

In scope:

- Escaping the sandbox: reading host files, reaching the network, affecting other executions.
- Bypassing limits on CPU time, wall-clock time, memory, process count, output size, or disk.
- Syscalls that the seccomp allowlist should deny but does not.
- Forging a grading result, for example making a failing submission report `PASSED`.
- Reading hidden test cases, from inside the sandbox or through the API.
- Authentication, session, or authorization flaws, including access to another user's or another
  course's data.

Out of scope:

- Vulnerabilities in the Linux kernel itself. The sandbox shares the host kernel, and the project
  makes no claim of protection against kernel bugs. Report those upstream.
- Volumetric denial-of-service against a deployed instance.
- Social engineering of team members or users.

## Testing guidelines

Test against **your own instance only**. Never run exploits against the production deployment,
which holds real student data and grades.
