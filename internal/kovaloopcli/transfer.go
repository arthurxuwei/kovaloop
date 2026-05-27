package kovaloopcli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strings"
)

type transferInput struct {
	ToEmail        string          `json:"toEmail"`
	Email          string          `json:"email"`
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
	FromEmail    string `json:"fromEmail"`
	ToEmail      string `json:"toEmail"`
	AmountAtomic string `json:"amountAtomic"`
	Reason       string `json:"reason"`
}

func runLedgerTransfer(args []string, stdout io.Writer, stderr io.Writer, cfg Config) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: kovaloop ledger transfer '<json>'")
		return 2
	}
	profile, err := LoadProfile(ProfilePath(cfg))
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	body, err := buildTransferRequest([]byte(args[0]), profile)
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

func buildTransferRequest(data []byte, profile Profile) (transferRequest, error) {
	var rawPayload map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawPayload); err != nil {
		return transferRequest{}, fmt.Errorf("transfer payload is malformed JSON: %s", err.Error())
	}
	if _, ok := rawPayload["fromAgentId"]; ok {
		return transferRequest{}, fmt.Errorf("fromAgentId/toAgentId are internal; use recipient email")
	}
	if _, ok := rawPayload["toAgentId"]; ok {
		return transferRequest{}, fmt.Errorf("fromAgentId/toAgentId are internal; use recipient email")
	}

	var payload transferInput
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return transferRequest{}, fmt.Errorf("transfer payload is malformed JSON: %s", err.Error())
	}

	fromEmail := normalizeEmail(profile.Email)
	if fromEmail == "" {
		return transferRequest{}, fmt.Errorf("current OpenClaw profile is missing email")
	}
	toEmail := normalizeEmail(payload.ToEmail)
	if toEmail == "" {
		toEmail = normalizeEmail(payload.Email)
	}
	if toEmail == "" {
		return transferRequest{}, fmt.Errorf("recipient email is required via toEmail or email")
	}
	if fromEmail == toEmail {
		return transferRequest{}, fmt.Errorf("sender and receiver emails must differ")
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
		FromEmail:    fromEmail,
		ToEmail:      toEmail,
		AmountAtomic: amountAtomic,
		Reason:       strings.TrimSpace(context.Reason),
	}, nil
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
