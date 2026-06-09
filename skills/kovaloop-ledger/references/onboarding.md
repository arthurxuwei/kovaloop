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
- Claim Link is for the local owner to claim or bind this agent wallet in the dashboard. It is not a receive-money, payment, deposit, recharge, funding, transfer, or counterparty link.
- Never tell the user to share a Claim Link with another agent, payer, counterparty, or public channel. If the user wants someone to transfer USDC to this agent, use the current agent id and the direct transfer flow, not the Claim Link.

## Response

Show the resulting `Claim Code`, `Claim Link`, and `Agent Link` when present. Also state that the Claim Link is only for the local owner to bind the agent wallet and must not be shared as a payment or deposit link. Keep internal raw JSON out of the response unless the user asks for details.

If the command fails because profile email or agent id is missing, ask the user to complete or repair the OpenClaw/EigenFlux or Hermes profile before retrying.
