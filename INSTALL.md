# Install Kovaloop CLI

Install the Go-based `kovaloop` CLI and the `kovaloop-ledger` skill.

Kovaloop installs into OpenClaw workspaces and Hermes config directories.

By default, run the installer from the directory that contains
`runtime-openclaw-*/workspace` or `runtime-hermes-*/config`:

```bash
curl -fsSL https://raw.githubusercontent.com/arthurxuwei/kovaloop/main/install.sh | bash
```

To install one workspace explicitly:

```bash
curl -fsSL https://raw.githubusercontent.com/arthurxuwei/kovaloop/main/install.sh \
  | OPENCLAW_WORKSPACE_DIR='/path/to/runtime-openclaw-x/workspace' bash
```

To install one Hermes config explicitly:

```bash
curl -fsSL https://raw.githubusercontent.com/arthurxuwei/kovaloop/main/install.sh \
  | HERMES_CONFIG_DIR='/path/to/runtime-hermes-x/config' bash
```

The installer is still the supported installation path. Normal users do not
need Go; `install.sh` downloads the platform binary from GitHub releases by
default using `KOVALOOP_INSTALL_BIN_BASE_URL`, which defaults to
`https://github.com/arthurxuwei/kovaloop/releases/latest/download`.

Supported release platforms:

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`

After installation, the installer mints the agent identity with
`kovaloop profile create` (idempotent) and then prints `Claim Link` and
`Agent Link` by running `kovaloop claim link`. Claim link reads the canonical
agent id from `.kovaloop/profile.json` and sends no email — ownership is bound
when the local owner opens the Claim Link and signs in with their email on the
web dashboard. If the ledger is unavailable, rerun:

```bash
KOVALOOP_HOME='/path/to/.openclaw' '/path/to/.local/bin/kovaloop' claim link
```

## Verify

On the host:

```bash
test -x /path/to/workspace/.local/bin/kovaloop
/path/to/workspace/.local/bin/kovaloop version
/path/to/workspace/.local/bin/kovaloop ledger health
/path/to/workspace/.local/bin/kovaloop ledger state
/path/to/workspace/.local/bin/kovaloop ledger route '{"deliveryMode":"agent_transfer","requiresAcceptance":false,"amountAtomic":"1","asset":"USDC"}'
/path/to/workspace/.local/bin/kovaloop ledger transfer '{"toAgentId":"agent_receiver","amount":"0.000001 U","paymentContext":{"source":"local_user_test","userApproved":true,"reason":"Local user asked this agent to run an online transfer test"}}'
```

The hosted Kovaloop service defaults are built into the `kovaloop` command. Override
them only under operator guidance or when pointing this install kit at another
deployment. The install docs intentionally avoid service path details; agents
should use the `kovaloop` commands instead of constructing backend calls.

For developer verification, run:

```bash
./scripts/build-release.sh
go test ./...
```

Ensure the OpenClaw workspace config allows the `kovaloop` command. Kovaloop skills
are installed under `workspace/skills`; set `skills.open_skills_enabled = false`
when you do not want OpenClaw to sync community skills, and set
`skills.allow_scripts = true` when local skills include shell scripts.

For Hermes, Kovaloop skills are installed under `config/skills` and the CLI is
installed under `config/bin`. Restart the Hermes gateway after installing if the
running agent does not pick up newly installed skills immediately.
