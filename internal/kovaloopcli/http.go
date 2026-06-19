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
	data, err := doRequest(http.MethodGet, cfg.LedgerURL, path, nil, nil)
	if err != nil {
		return err
	}
	return decodeJSONResponse(data, out)
}

func getRaw(cfg Config, path string) ([]byte, error) {
	return doRequest(http.MethodGet, cfg.LedgerURL, path, nil, nil)
}

func postJSON(cfg Config, path string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	data, err := doRequest(http.MethodPost, cfg.LedgerURL, path, payload, nil)
	if err != nil {
		return err
	}
	return decodeJSONResponse(data, out)
}

func postRaw(cfg Config, path string, body any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return doRequest(http.MethodPost, cfg.LedgerURL, path, payload, nil)
}

func postRawJSON(cfg Config, path string, body json.RawMessage) ([]byte, error) {
	return doRequest(http.MethodPost, cfg.LedgerURL, path, body, nil)
}

// patchRaw sends a PATCH with the exact body bytes and caller-supplied headers
// (used for signed agent requests, where the signed bytes must be transmitted unchanged).
func patchRaw(cfg Config, path string, body []byte, headers map[string]string) ([]byte, error) {
	return doRequest(http.MethodPatch, cfg.LedgerURL, path, body, headers)
}

func doRequest(method string, base string, path string, body []byte, headers map[string]string) ([]byte, error) {
	url := strings.TrimRight(base, "/") + path
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ledger response read failed: %s", err.Error())
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ledger request failed: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
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
