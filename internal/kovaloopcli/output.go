package kovaloopcli

import (
	"fmt"
	"io"
)

type ClaimResponse struct {
	AgentID   string `json:"agentId"`
	ClaimCode string `json:"claimCode"`
	ClaimURL  string `json:"claimUrl"`
	AgentURL  string `json:"agentUrl"`
}

func printClaimResponse(w io.Writer, response ClaimResponse) {
	fmt.Fprintf(w, "Agent ID:   %s\n", response.AgentID)
	fmt.Fprintf(w, "Claim Code: %s\n", response.ClaimCode)
	fmt.Fprintf(w, "Claim Link: %s\n", response.ClaimURL)
	fmt.Fprintf(w, "Agent Link: %s\n", response.AgentURL)
	fmt.Fprintln(w, "Claim Link is for the local owner to bind this agent wallet. Do not share it as a payment or deposit link.")
}
