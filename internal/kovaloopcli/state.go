package kovaloopcli

import (
	"encoding/json"
	"fmt"
	"net/url"
)

type ledgerAccountResponse struct {
	Account map[string]any `json:"account"`
}

type ledgerEntriesResponse struct {
	Entries []map[string]any `json:"entries"`
}

type ledgerOnrampSessionsResponse struct {
	OnrampSessions []map[string]any `json:"onrampSessions"`
}

type ledgerStatePayload struct {
	Accounts            []map[string]any `json:"accounts"`
	Entries             []map[string]any `json:"entries"`
	OnrampSessions      []map[string]any `json:"onrampSessions"`
	OnrampEvents        []map[string]any `json:"onrampEvents"`
	CircleWebhookEvents []map[string]any `json:"circleWebhookEvents"`
	ChainRecords        []map[string]any `json:"chainRecords"`
	SettlementRecords   []map[string]any `json:"settlementRecords"`
}

func LedgerState(cfg Config) ([]byte, error) {
	profile, err := LoadProfile(ProfilePath(cfg))
	if err != nil {
		return nil, err
	}
	agentID := profile.normalizedAgentID()
	if agentID == "" {
		return nil, fmt.Errorf("current OpenClaw profile is missing agent_id")
	}

	escapedPathAgentID := url.PathEscape(agentID)
	escapedQueryAgentID := url.QueryEscape(agentID)

	var accountResponse ledgerAccountResponse
	if err := getJSON(cfg, "/ledger/accounts/"+escapedPathAgentID, &accountResponse); err != nil {
		return nil, err
	}
	if accountResponse.Account == nil {
		return nil, fmt.Errorf("ledger state response is missing expected domain fields")
	}

	var entriesResponse ledgerEntriesResponse
	if err := getJSON(cfg, "/ledger/accounts/"+escapedPathAgentID+"/entries?limit=500", &entriesResponse); err != nil {
		return nil, err
	}
	if entriesResponse.Entries == nil {
		return nil, fmt.Errorf("ledger state response is missing expected domain fields")
	}

	var onrampSessionsResponse ledgerOnrampSessionsResponse
	if err := getJSON(cfg, "/ledger/onramp-sessions?agentId="+escapedQueryAgentID+"&limit=500", &onrampSessionsResponse); err != nil {
		return nil, err
	}
	if onrampSessionsResponse.OnrampSessions == nil {
		return nil, fmt.Errorf("ledger state response is missing expected domain fields")
	}

	state := ledgerStatePayload{
		Accounts:            []map[string]any{accountResponse.Account},
		Entries:             entriesResponse.Entries,
		OnrampSessions:      onrampSessionsResponse.OnrampSessions,
		OnrampEvents:        []map[string]any{},
		CircleWebhookEvents: []map[string]any{},
		ChainRecords:        []map[string]any{},
		SettlementRecords:   []map[string]any{},
	}
	sanitizeAvailableAtomic(state.Accounts)
	sanitizeAvailableAtomic(state.Entries)
	sanitizeAvailableAtomic(state.OnrampSessions)
	return json.Marshal(state)
}

func sanitizeAvailableAtomic(value any) {
	switch typed := value.(type) {
	case map[string]any:
		delete(typed, "availableAtomic")
		for _, child := range typed {
			sanitizeAvailableAtomic(child)
		}
	case []map[string]any:
		for _, child := range typed {
			sanitizeAvailableAtomic(child)
		}
	case []any:
		for _, child := range typed {
			sanitizeAvailableAtomic(child)
		}
	}
}
