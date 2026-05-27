package kovaloopcli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"strings"
	"time"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		ResponseHeaderTimeout: 10 * time.Second,
	},
}

func getJSON(cfg Config, path string, out any) error {
	return doJSON(http.MethodGet, cfg.LedgerURL, cfg.LedgerFallback, path, nil, out)
}

func getRaw(cfg Config, path string) ([]byte, error) {
	return doRaw(http.MethodGet, cfg.LedgerURL, cfg.LedgerFallback, path, nil)
}

func postJSON(cfg Config, path string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return doJSON(http.MethodPost, cfg.LedgerURL, cfg.LedgerFallback, path, payload, out)
}

func postRaw(cfg Config, path string, body any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return doRaw(http.MethodPost, cfg.LedgerURL, cfg.LedgerFallback, path, payload)
}

func postRawJSON(cfg Config, path string, body json.RawMessage) ([]byte, error) {
	return doRaw(http.MethodPost, cfg.LedgerURL, cfg.LedgerFallback, path, body)
}

func doJSON(method string, primary string, fallback string, path string, body []byte, out any) error {
	data, err := doRaw(method, primary, fallback, path, body)
	if err != nil {
		return err
	}
	return decodeJSONResponse(data, out)
}

func doRaw(method string, primary string, fallback string, path string, body []byte) ([]byte, error) {
	data, retryable, err := doJSONOnce(method, primary, path, body)
	if err != nil {
		if !retryable || fallback == "" {
			return nil, err
		}
		data, _, err = doJSONOnce(method, fallback, path, body)
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}

func doJSONOnce(method string, base string, path string, body []byte) ([]byte, bool, error) {
	url := strings.TrimRight(base, "/") + path
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, true, err
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, fmt.Errorf("ledger response read failed: %s", err.Error())
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, true, fmt.Errorf("ledger request failed: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, false, nil
}

func decodeJSONResponse(data []byte, out any) error {
	if out == nil {
		return nil
	}
	if len(data) == 0 {
		return nil
	}
	target := reflect.ValueOf(out)
	if target.Kind() != reflect.Ptr || target.IsNil() {
		return json.Unmarshal(data, out)
	}
	temp := reflect.New(target.Elem().Type())
	if err := json.Unmarshal(data, temp.Interface()); err != nil {
		return fmt.Errorf("ledger response was not valid JSON: %s", err.Error())
	}
	target.Elem().Set(temp.Elem())
	return nil
}
