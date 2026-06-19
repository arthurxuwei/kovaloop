package kovaloopcli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"strings"
)

type transferInput struct {
	ToAgentID      string          `json:"toAgentId"`
	ToEmail        string          `json:"toEmail"`
	Amount         json.RawMessage `json:"amount"`
	AmountAtomic   json.RawMessage `json:"amountAtomic"`
	PaymentContext json.RawMessage `json:"paymentContext"`
}

type transferPaymentContext struct {
	Source       string `json:"source"`
	UserApproved bool   `json:"userApproved"`
	Reason       string `json:"reason"`
}

type transferRequest struct {
	FromAgentID  string `json:"fromAgentId"`
	ToAgentID    string `json:"toAgentId"`
	AmountAtomic string `json:"amountAtomic"`
	Reason       string `json:"reason"`
}

func runLedgerTransfer(args []string, stdout io.Writer, stderr io.Writer, cfg Config) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: kovaloop ledger transfer '<json>'")
		return 2
	}
	fromAgentID, err := CanonicalAgentID(cfg)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	body, err := buildTransferRequestWithResolver(
		[]byte(args[0]),
		fromAgentID,
		func(email string) (string, error) {
			return resolveRecipientAgentIDByEmail(cfg, email)
		},
	)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	response, err := postRaw(cfg, "/ledger/transfers", body)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	printRawResponse(stdout, response)
	return 0
}

func buildTransferRequest(data []byte, fromAgentID string) (transferRequest, error) {
	return buildTransferRequestWithResolver(data, fromAgentID, nil)
}

func buildTransferRequestWithResolver(
	data []byte,
	fromAgentID string,
	resolveRecipientEmail func(string) (string, error),
) (transferRequest, error) {
	var rawPayload map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawPayload); err != nil {
		return transferRequest{}, fmt.Errorf("transfer payload is malformed JSON: %s", err.Error())
	}
	if _, ok := rawPayload["fromEmail"]; ok {
		return transferRequest{}, fmt.Errorf("fromEmail is no longer accepted; sender is resolved from the current profile")
	}
	if _, ok := rawPayload["email"]; ok {
		return transferRequest{}, fmt.Errorf("email is ambiguous; use toEmail for recipient lookup or toAgentId for direct transfer")
	}
	if _, ok := rawPayload["fromAgentId"]; ok {
		return transferRequest{}, fmt.Errorf("fromAgentId is resolved from the current profile")
	}

	var payload transferInput
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return transferRequest{}, fmt.Errorf("transfer payload is malformed JSON: %s", err.Error())
	}

	fromAgentID = strings.TrimSpace(fromAgentID)
	if fromAgentID == "" {
		return transferRequest{}, fmt.Errorf("no local KovaLoop profile; run 'kovaloop profile create' first")
	}
	toAgentID := strings.TrimSpace(payload.ToAgentID)
	toEmail := normalizeEmail(payload.ToEmail)
	if toAgentID != "" && toEmail != "" {
		return transferRequest{}, fmt.Errorf("provide either toAgentId or toEmail, not both")
	}
	if toAgentID == "" && toEmail != "" {
		if resolveRecipientEmail == nil {
			return transferRequest{}, fmt.Errorf("recipient agent id is required via toAgentId")
		}
		resolvedAgentID, err := resolveRecipientEmail(toEmail)
		if err != nil {
			return transferRequest{}, err
		}
		toAgentID = strings.TrimSpace(resolvedAgentID)
	}
	if toAgentID == "" {
		return transferRequest{}, fmt.Errorf("recipient agent id is required via toAgentId")
	}
	if fromAgentID == toAgentID {
		return transferRequest{}, fmt.Errorf("sender and receiver agent ids must differ")
	}

	amountAtomic, err := transferAmountAtomic(payload)
	if err != nil {
		return transferRequest{}, err
	}
	context, err := validateTransferPaymentContext(payload.PaymentContext)
	if err != nil {
		return transferRequest{}, err
	}

	if err := validateAgentTransferRoute(); err != nil {
		return transferRequest{}, err
	}
	return transferRequest{
		FromAgentID:  fromAgentID,
		ToAgentID:    toAgentID,
		AmountAtomic: amountAtomic,
		Reason:       strings.TrimSpace(context.Reason),
	}, nil
}

type transferAccountListResponse struct {
	Accounts []map[string]any `json:"accounts"`
}

func resolveRecipientAgentIDByEmail(cfg Config, email string) (string, error) {
	normalizedEmail := normalizeEmail(email)
	if normalizedEmail == "" {
		return "", fmt.Errorf("recipient email is empty")
	}

	var response transferAccountListResponse
	path := "/ledger/accounts?ownerEmail=" + url.QueryEscape(normalizedEmail)
	if err := getJSON(cfg, path, &response); err != nil {
		return "", err
	}
	if response.Accounts == nil {
		return "", fmt.Errorf("ledger account lookup response is missing accounts")
	}

	agentIDs := []string{}
	for _, account := range response.Accounts {
		if agentID, ok := stringField(account, "agentId"); ok {
			agentIDs = append(agentIDs, agentID)
		}
	}
	if len(agentIDs) == 0 {
		return "", fmt.Errorf("no Kovaloop agent found for recipient email %s; ask for the recipient agent id", normalizedEmail)
	}
	if len(agentIDs) > 1 {
		return "", fmt.Errorf("multiple Kovaloop agents found for recipient email %s; ask for the recipient agent id", normalizedEmail)
	}
	return agentIDs[0], nil
}

func transferAmountAtomic(payload transferInput) (string, error) {
	if len(payload.AmountAtomic) != 0 {
		return parseAtomicAmountValue(payload.AmountAtomic)
	}
	if len(payload.Amount) != 0 {
		return parseTransferAmountValue(payload.Amount)
	}
	return "", fmt.Errorf("amount is required")
}

func parseTransferAmountValue(raw json.RawMessage) (string, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", fmt.Errorf("invalid amount")
	}
	switch v := value.(type) {
	case string:
		return parseTransferAmount(v)
	case json.Number:
		return parseTransferAmount(v.String())
	default:
		return "", fmt.Errorf("invalid amount")
	}
}

func parseAtomicAmountValue(raw json.RawMessage) (string, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", fmt.Errorf("invalid amountAtomic")
	}
	switch v := value.(type) {
	case string:
		return parsePositiveAtomicAmount(v, "amountAtomic")
	case json.Number:
		return parsePositiveAtomicAmount(v.String(), "amountAtomic")
	default:
		return "", fmt.Errorf("amountAtomic must be a positive integer")
	}
}

func parsePositiveAtomicAmount(raw string, field string) (string, error) {
	value := strings.TrimSpace(raw)
	if strings.HasPrefix(value, "-") {
		return "", fmt.Errorf("%s must be a positive integer", field)
	}
	if !allDigits(value) {
		return "", fmt.Errorf("%s must be a positive integer", field)
	}
	amount := new(big.Int)
	amount.SetString(value, 10)
	if amount.Sign() <= 0 {
		return "", fmt.Errorf("%s must be a positive integer", field)
	}
	return amount.String(), nil
}

func validateTransferPaymentContext(raw json.RawMessage) (transferPaymentContext, error) {
	if len(raw) == 0 {
		return transferPaymentContext{}, fmt.Errorf("transfer requires paymentContext")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return transferPaymentContext{}, fmt.Errorf("paymentContext must be an object")
	}
	approved, ok := object["userApproved"]
	if !ok || string(bytes.TrimSpace(approved)) != "true" {
		return transferPaymentContext{}, fmt.Errorf("paymentContext.userApproved must be true")
	}

	var context transferPaymentContext
	if err := json.Unmarshal(raw, &context); err != nil {
		return transferPaymentContext{}, fmt.Errorf("paymentContext is invalid: %s", err.Error())
	}
	if context.Source != "local_user_request" && context.Source != "local_user_test" {
		return transferPaymentContext{}, fmt.Errorf("paymentContext.source must be local_user_request or local_user_test")
	}
	if strings.TrimSpace(context.Reason) == "" {
		return transferPaymentContext{}, fmt.Errorf("paymentContext.reason is required")
	}
	return context, nil
}

func validateAgentTransferRoute() error {
	var decision routeDecision
	if err := json.Unmarshal([]byte(RoutePaymentIntent(`{"deliveryMode":"agent_transfer"}`)), &decision); err != nil {
		return fmt.Errorf("agent_transfer route failed: %s", err.Error())
	}
	if decision.NeedsClarification || !containsString(decision.AllowedTools, "agent_wallet_transfer") {
		return fmt.Errorf("agent_transfer route did not allow agent_wallet_transfer")
	}
	return nil
}

func parseTransferAmount(raw string) (string, error) {
	value, unit, ok := splitAmountAndUnit(raw)
	if !ok || value == "" {
		return "", fmt.Errorf("invalid amount")
	}
	if strings.HasPrefix(value, "-") {
		return "", fmt.Errorf("amount must be greater than zero")
	}
	if strings.HasPrefix(value, "+") {
		return "", fmt.Errorf("invalid amount")
	}
	if strings.Contains(value, ".") || unit != "" {
		return parseUSDCAmount(value, unit)
	}
	if !allDigits(value) {
		return "", fmt.Errorf("invalid amount")
	}
	amount := new(big.Int)
	amount.SetString(value, 10)
	if amount.Sign() <= 0 {
		return "", fmt.Errorf("amount must be greater than zero")
	}
	return amount.String(), nil
}

func parseUSDCAmount(value string, unit string) (string, error) {
	if unit != "" && unit != "U" && unit != "USDC" {
		return "", fmt.Errorf("invalid amount")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return "", fmt.Errorf("invalid amount")
	}
	whole := parts[0]
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	if !allDigits(whole) || (frac != "" && !allDigits(frac)) {
		return "", fmt.Errorf("invalid amount")
	}
	if len(frac) > 6 {
		for _, r := range frac[6:] {
			if r != '0' {
				return "", fmt.Errorf("sub-atomic decimal amount is not allowed")
			}
		}
		frac = frac[:6]
	}
	for len(frac) < 6 {
		frac += "0"
	}
	amount := new(big.Int)
	amount.SetString(whole+frac, 10)
	if amount.Sign() <= 0 {
		return "", fmt.Errorf("amount must be greater than zero")
	}
	return amount.String(), nil
}

func splitAmountAndUnit(raw string) (string, string, bool) {
	trimmed := strings.TrimSpace(raw)
	fields := strings.Fields(trimmed)
	switch len(fields) {
	case 1:
		value, unit := splitAttachedAmountUnit(fields[0])
		return value, unit, true
	case 2:
		return fields[0], strings.ToUpper(fields[1]), true
	default:
		return "", "", false
	}
}

func splitAttachedAmountUnit(raw string) (string, string) {
	upper := strings.ToUpper(raw)
	for _, unit := range []string{"USDC", "U"} {
		if strings.HasSuffix(upper, unit) {
			return raw[:len(raw)-len(unit)], unit
		}
	}
	return raw, ""
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
