# Payment Routing

Use this reference before any funding, payment, transfer, x402-like, or value-changing action.

## Rule

Route payment intent first. After routing, use only the returned `allowedTools` / command family. If routing returns `needs_clarification`, ask the user before funding or paying.

## Command

```bash
kovaloop ledger route '{"deliveryMode":"agent_transfer","requiresAcceptance":false,"amountAtomic":"1000000","asset":"USDC"}'
```

Adjust the JSON intent to match the user's actual request. Keep the intent focused on the requested payment or funding action.

## Denied Or Unsupported Paths

Do not use this skill to perform:

- Ledger credit writes
- Direct Circle wallet lifecycle or settlement operations
- Direct chain signing, transfer, execution, or UserOperation actions
- x402 paid fetches
- Hidden settlement workflows not documented by the public Kovaloop skill

If the router returns an allowed command family that this skill does not document, stop and ask the user or operator for the relevant Kovaloop skill/instructions before acting.

## Clarification

Ask a clarification when the router says it needs one, or when the local user's request does not specify the recipient, amount, asset, or whether a real payment/test transfer is authorized.
