# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Repository scaffolding: README, MIT license, contribution guide, security policy, and component
  directories.
- GitHub Actions CI. Each component's jobs activate once that component contains code.
- Issue templates for bug reports and user stories, and a pull request template.
- Weekly progress and sprint review templates under `docs/`.
- Sandbox Executor (`sandbox/`): `sandbox-exec` runs one command in user, PID, mount, network, UTS
  and IPC namespaces with cgroup v2 limits, a seccomp allowlist that reports the denied syscall
  number, a deterministic environment and a JSON result channel, plus `--sweep` for crash leftovers
  and an adversarial test suite (`make test`, `make asan`). Ships a man page and `make install`.
