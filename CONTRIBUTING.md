# Contributing

This guide covers how the team works in this repository: branches, commits, pull requests, and the
quality gates each component must pass.

## Branching model

| Branch | Purpose | Deploys to |
|---|---|---|
| `main` | Stable, released code. Changes arrive only by pull request from `develop`. | Production |
| `develop` | Integration branch. Every feature branch merges here first. | Staging |
| `feature/<short-description>` | One feature or user story, branched from `develop`. | — |
| `fix/<short-description>` | One bug fix, branched from `develop`. | — |

Workflow:

1. Branch from an up-to-date `develop`: `git switch develop && git pull && git switch -c feature/ws-ticket-auth`.
2. Keep the branch focused on one issue. Rebase on `develop` if it falls behind.
3. Open a pull request into `develop` and fill in the template.
4. At least **one other team member** approves. CI must pass.
5. Merge with **Squash and merge**. The squashed commit uses the PR title, so the title must follow
   the commit convention below.
6. To release, open a PR from `develop` into `main`. After merging, tag the release (`vX.Y.Z`,
   [Semantic Versioning](https://semver.org/)) and move the `Unreleased` entries in
   [CHANGELOG.md](CHANGELOG.md) under the new version.

## Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/). Write the scope as the component name:

```
<type>(<scope>): <imperative summary, lowercase, no trailing period>
```

| Type | Use for |
|---|---|
| `feat` | New user-visible behaviour |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `test` | Adding or fixing tests |
| `chore` | Tooling, CI, dependencies, repository maintenance |

Scopes: `sandbox`, `scheduler`, `grader`, `frontend`, `infra`, `docs`, `ci`.

```
feat(sandbox): enforce combined stdout+stderr output limit
fix(scheduler): return 404 instead of 403 for cross-course assignments
test(grader): cover whitespace-insensitive match mode
```

## Pull requests

- Link the issue (`Closes #42`) and name the requirement IDs the change implements (for example
  `FR-04`, `NFR-10`). The QA traceability matrix is built from these references.
- The sandbox backup reviews every change under `sandbox/`, so more than one person can maintain it.
- Changes to authentication, authorization, sessions, or the sandbox are security-sensitive. The
  reviewer checks the negative cases, not only the happy path.
- Update the component README, the API documentation, and `CHANGELOG.md` when behaviour changes.

## Quality gates

CI runs lint, tests, and build for every component that has code. Run the same checks locally
before you push.

| Component | Required |
|---|---|
| `sandbox/` (C) | Compiles with `-Wall -Wextra -Werror`. `clang-tidy` reports no findings. The adversarial suite passes under AddressSanitizer and UndefinedBehaviorSanitizer. Critical functions document the syscalls and flags they use. |
| `scheduler/` (Go) | `gofmt` and `go vet` are clean. Tests pass with `-race`. Scheduler and API handlers keep **≥ 70 %** coverage. |
| `grader/` (Python) | `ruff check` is clean (PEP 8). `pytest` passes. |
| `frontend/` | Lint, tests (when present), and production build succeed. Layouts work at 360 px, 768 px, and ≥ 1280 px. Forms work with the keyboard and carry accessible labels. |

## Secrets and private data

The repository is **public**.

- Never commit `.env` files, credentials, SSH keys, or tokens. Deployment secrets live in GitHub
  Actions secrets, separately for each environment.
- Keep real assignment test cases (**especially hidden ones**) and seed data in `/private/`, which
  is git-ignored. Only synthetic fixtures written for automated tests belong in the repository.
- If a secret is committed, treat it as leaked. Rotate it immediately. Removing it from history is
  not enough.

## Project documentation

Record progress while the work happens, not at the end:

- Weekly progress note: `docs/progress/minggu-NN.md`, created from
  [`docs/progress/_template.md`](docs/progress/_template.md).
- Sprint review and retrospective: `docs/sprint/sprint-N.md`, created from
  [`docs/sprint/_template.md`](docs/sprint/_template.md).
- Keep the GitHub Projects board (Backlog → To Do → In Progress → Review → Done) in step with reality.

## Repository settings (maintainers)

Apply these once the repository is on GitHub:

- **Branch protection** on `main` and `develop`: require a pull request, one approval, passing CI
  status checks, and up-to-date branches. Block force pushes.
- **Security**: enable private vulnerability reporting, secret scanning with push protection, and
  Dependabot alerts.
- **Merge options**: allow squash merging only.
