# Balance And Ledger State

Use this reference for balance, "查余额", "到账了吗", funding/onramp status, ledger health, and visible wallet state.

## Health

```bash
kovaloop ledger health
```

Use health before deeper debugging when the user asks whether the ledger service is reachable or working.

## State

```bash
kovaloop ledger state
```

This command reads the local OpenClaw/Hermes profile and returns the ledger view scoped to the current agent. If no profile agent id is available, do not treat state as global account data.

## Balance Rules

- Agent-visible available balance is Circle-sourced by the service.
- Do not label any balance as "Ledger available balance".
- Do not list other accounts.
- Do not ask the user to choose a source account.
- Do not infer sender or wallet identity from the first account in ledger state.
- Never invent settlement, funding, onramp, pending, or released status. Use command output.
- USDC atomic amounts use 6 decimals. Prefer `amountDisplay` and `availableDeltaDisplay` from `kovaloop ledger state`; otherwise convert atomic integers exactly with 1 USDC = 1000000 atomic units. For example, one atomic unit is `0.000001 USDC`. Never report a non-zero atomic amount as `0 USDC`.

## Response

Summarize only the current agent's visible balance and wallet-related status. If the user asks for details, include concise fields from the command output; otherwise avoid raw JSON.
