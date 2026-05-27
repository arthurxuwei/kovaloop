package kovaloopcli

import "encoding/json"

type paymentIntent struct {
	DeliveryMode       string `json:"deliveryMode"`
	RequiresAcceptance bool   `json:"requiresAcceptance"`
}

type routeDecision struct {
	Method             string   `json:"method"`
	NeedsClarification bool     `json:"needsClarification"`
	AllowedTools       []string `json:"allowedTools"`
	Reason             string   `json:"reason"`
}

func RoutePaymentIntent(intentJSON string) string {
	var intent paymentIntent
	_ = json.Unmarshal([]byte(intentJSON), &intent)

	var decision routeDecision
	switch {
	case intent.DeliveryMode == "funding":
		decision = routeDecision{
			Method:             "onramp",
			NeedsClarification: false,
			AllowedTools:       []string{"agent_wallet_create_onramp_session"},
			Reason:             "Funding must create a hosted onramp session; ledger balance is credited only after provider-confirmed settlement.",
		}
	case intent.DeliveryMode == "agent_transfer":
		decision = routeDecision{
			Method:             "gateway_nanopayment",
			NeedsClarification: false,
			AllowedTools:       []string{"agent_wallet_transfer"},
			Reason:             "Immediate internal Agent-to-Agent payments use Circle Gateway Nanopayments; the ledger records the transfer only after Gateway settlement succeeds.",
		}
	case intent.DeliveryMode == "withdrawal":
		decision = routeDecision{
			Method:             "needs_clarification",
			NeedsClarification: true,
			AllowedTools:       []string{},
			Reason:             "This install kit only exposes ledger wallet, funding, direct transfer, and settlement operations. Ask the operator before handling withdrawals.",
		}
	case intent.DeliveryMode == "immediate_api":
		decision = routeDecision{
			Method:             "needs_clarification",
			NeedsClarification: true,
			AllowedTools:       []string{},
			Reason:             "This install kit only exposes ledger wallet, funding, direct transfer, and settlement operations. Ask the operator before immediate paid API calls.",
		}
	case intent.DeliveryMode == "async_task" || intent.RequiresAcceptance:
		decision = routeDecision{
			Method:             "needs_clarification",
			NeedsClarification: true,
			AllowedTools:       []string{},
			Reason:             "Asynchronous task settlement is not publicly exposed in this install kit. Ask the operator before handling this payment.",
		}
	default:
		decision = routeDecision{
			Method:             "needs_clarification",
			NeedsClarification: true,
			AllowedTools:       []string{},
			Reason:             "The payment intent is ambiguous. Clarify whether this is funding, direct transfer, withdrawal, or another payment type before proceeding.",
		}
	}

	data, err := json.Marshal(decision)
	if err != nil {
		return `{"method":"needs_clarification","needsClarification":true,"allowedTools":[],"reason":"The payment intent is ambiguous. Clarify whether this is funding, direct transfer, withdrawal, or another payment type before proceeding."}` + "\n"
	}
	return string(data) + "\n"
}
