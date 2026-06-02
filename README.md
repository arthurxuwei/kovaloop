# kovaloop

Install kit for exposing Kovaloop ledger capabilities to OpenClaw/Hermes-style
agent runtimes. The repository distributes a Go-based `kovaloop` CLI plus the
Kovaloop skills needed by agent workspaces.

This repository intentionally contains only the distribution artifacts an agent
needs:

- `cmd/kovaloop`: Go CLI source for the local `kovaloop` entrypoint used by agents
- `skills/kovaloop-ledger`: ledger, Agent Wallet onboarding, direct Agent transfer, and funding state skill
- `install.sh`: curl-pipe installer for OpenClaw and Hermes runtime files
- `INSTALL.md`: install and verification steps

See [INSTALL.md](INSTALL.md) for installation.

Normal users install prebuilt binaries through `install.sh` and do not need Go
installed. Supported platforms, binary download settings, and developer
verification commands are documented in [INSTALL.md](INSTALL.md).

Hosted service defaults live in `kovaloop`; override them with `KOVALOOP_*`
environment variables only when using another deployment.
