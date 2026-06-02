# Agent Wallet Onboarding

Use this reference for install follow-up, reinstall follow-up, `claimCode`, claim code, claim link, agent link, wallet binding, or "领取钱包" requests.

## Command

```bash
kovaloop claim link
```

## Behavior

- Run this immediately after a successful Kovaloop installation or reinstall.
- Run this whenever the user asks for `claimCode`, a claim code, Claim Link, Agent Link, wallet onboarding, wallet binding, or how to claim the installed agent.
- Do not answer from memory. Claim codes and links are runtime state.
- The command reads the current OpenClaw/Hermes profile, creates or reuses the backend wallet binding, ensures the corresponding zero-balance ledger account exists, and prints `Agent ID`, `Claim Code`, `Claim Link`, and `Agent Link`.
- The owner email comes from the current OpenClaw/Hermes profile and must never be omitted, guessed, or taken from another ledger account.

## Response

Show the resulting `Claim Code`, `Claim Link`, and `Agent Link` when present. Keep internal raw JSON out of the response unless the user asks for details.

If the command fails because profile email or agent id is missing, ask the user to complete or repair the OpenClaw/EigenFlux or Hermes profile before retrying.
