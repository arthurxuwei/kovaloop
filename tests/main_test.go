package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

var (
	repoRoot      string
	testKovaloop  string
	testAssetDir  string
	testAssetPath string
)

func TestMain(m *testing.M) {
	var err error
	repoRoot, err = filepath.Abs("..")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	testKovaloop = filepath.Join(repoRoot, "dist", "test", "kovaloop")
	if err := buildKovaloop(testKovaloop); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	testAssetDir, err = os.MkdirTemp("", "kovaloop-test-assets-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	testAssetPath = filepath.Join(testAssetDir, currentKovaloopAssetName())
	if err := buildKovaloop(testAssetPath); err != nil {
		_ = os.RemoveAll(testAssetDir)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	code := m.Run()
	_ = os.RemoveAll(testAssetDir)
	os.Exit(code)
}

func buildKovaloop(out string) error {
	cmd := exec.Command(filepath.Join(repoRoot, "scripts", "build-kovaloop.sh"), out)
	cmd.Dir = repoRoot
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build kovaloop %s: %w\n%s", out, err, output.String())
	}
	return nil
}

func currentKovaloopAssetName() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	if goos != "darwin" && goos != "linux" {
		fmt.Fprintf(os.Stderr, "unsupported test platform: %s/%s\n", goos, goarch)
		os.Exit(1)
	}
	if goarch != "amd64" && goarch != "arm64" {
		fmt.Fprintf(os.Stderr, "unsupported test platform: %s/%s\n", goos, goarch)
		os.Exit(1)
	}
	return "kovaloop_" + goos + "_" + goarch
}

func writeFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, string(data))
}

func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{repoRoot}, parts...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func asString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func removeEnv(env []string, key string) []string {
	prefix := key + "="
	next := make([]string, 0, len(env))
	for _, item := range env {
		if !strings.HasPrefix(item, prefix) {
			next = append(next, item)
		}
	}
	return next
}

func assertJSONMapEqual(t *testing.T, got map[string]any, want map[string]any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		gotJSON, _ := json.MarshalIndent(got, "", "  ")
		wantJSON, _ := json.MarshalIndent(want, "", "  ")
		t.Fatalf("map mismatch\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}
