package kovaloopcli

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"strings"
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
	addUSDCDisplayFields(state.Entries)
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

func addUSDCDisplayFields(entries []map[string]any) {
	for _, entry := range entries {
		if _, exists := entry["amountDisplay"]; !exists {
			if amountAtomic, ok := stringField(entry, "amountAtomic"); ok {
				if display, ok := formatAtomicUSDC(amountAtomic, true); ok {
					entry["amountDisplay"] = display
				}
			} else if availableDeltaAtomic, ok := stringField(entry, "availableDeltaAtomic"); ok {
				if display, ok := formatAtomicUSDC(availableDeltaAtomic, true); ok {
					entry["amountDisplay"] = display
				}
			}
		}
		if _, exists := entry["availableDeltaDisplay"]; !exists {
			if availableDeltaAtomic, ok := stringField(entry, "availableDeltaAtomic"); ok {
				if display, ok := formatAtomicUSDC(availableDeltaAtomic, false); ok {
					entry["availableDeltaDisplay"] = display
				}
			}
		}
	}
}

func stringField(record map[string]any, key string) (string, bool) {
	value, ok := record[key]
	if !ok {
		return "", false
	}
	switch typed := value.(type) {
	case string:
		text := strings.TrimSpace(typed)
		return text, text != ""
	case float64:
		if typed != float64(int64(typed)) {
			return "", false
		}
		return fmt.Sprintf("%.0f", typed), true
	default:
		return "", false
	}
}

func formatAtomicUSDC(value string, absolute bool) (string, bool) {
	text := strings.TrimSpace(value)
	if text == "" {
		return "", false
	}
	negative := strings.HasPrefix(text, "-")
	if negative || strings.HasPrefix(text, "+") {
		text = text[1:]
	}
	if text == "" {
		return "", false
	}
	atomic := new(big.Int)
	if _, ok := atomic.SetString(text, 10); !ok {
		return "", false
	}
	oneUSDC := big.NewInt(1_000_000)
	whole := new(big.Int).Quo(atomic, oneUSDC)
	fraction := new(big.Int).Rem(atomic, oneUSDC)
	prefix := ""
	if negative && !absolute && atomic.Sign() != 0 {
		prefix = "-"
	}
	return fmt.Sprintf("%s%s.%06d", prefix, whole.String(), fraction.Int64()), true
}
