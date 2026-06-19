# Agent Wallet Onboarding

Use this reference for install follow-up, reinstall follow-up, `claimCode`, claim code, claim link, agent link, wallet binding, or "领取钱包" requests.

## Commands

```bash
kovaloop profile create   # mint/reuse the agent identity (idempotent)
kovaloop claim link
```

## Behavior

- Identity first: `kovaloop profile create` mints the canonical `kloop_agent_` identity, writes `.kovaloop/{profile,credentials}.json` to the durable config volume, and provisions the agent's wallet. It is idempotent — re-running reuses the existing identity and does nothing. The installer runs it automatically after install/reinstall.
- Run `kovaloop claim link` immediately after a successful install/reinstall, or whenever the user asks for `claimCode`, a claim code, Claim Link, Agent Link, wallet onboarding, wallet binding, or how to claim the installed agent.
- Do not answer from memory. Claim codes and links are runtime state.
- `kovaloop claim link` reads the canonical agent id from `.kovaloop/profile.json`, looks up the existing wallet/account, and prints `Agent ID`, `Claim Code`, `Claim Link`, and `Agent Link`. It does not collect or send an email.
- Ownership is bound when the local owner opens the Claim Link and signs in with their email on the web dashboard — not from any local profile. Do not ask the user for an email to run claim link.
- Claim Link is for the local owner to claim or bind this agent wallet in the dashboard. It is not a receive-money, payment, deposit, recharge, funding, transfer, or counterparty link.
- Never tell the user to share a Claim Link with another agent, payer, counterparty, or public channel. If the user wants someone to transfer USDC to this agent, use the current agent id and the direct transfer flow, not the Claim Link.

## Response

Show the resulting `Claim Code`, `Claim Link`, and `Agent Link` when present. Also state that the Claim Link is only for the local owner to bind the agent wallet and must not be shared as a payment or deposit link. Keep internal raw JSON out of the response unless the user asks for details.

If a command reports "no local KovaLoop profile", run `kovaloop profile create` first, then retry.
