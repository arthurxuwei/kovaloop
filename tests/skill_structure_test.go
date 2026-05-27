package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKovaloopLedgerSkillUsesProgressiveDisclosure(t *testing.T) {
	skill := readRepoFile(t, "skills", "kovaloop-ledger", "SKILL.md")

	for _, want := range []string{
		"references/onboarding.md",
		"references/balance-state.md",
		"references/payment-routing.md",
		"references/direct-transfer.md",
		"references/troubleshooting.md",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("SKILL.md does not reference %s", want)
		}
		if _, err := os.Stat(filepath.Join(repoRoot, "skills", "kovaloop-ledger", want)); err != nil {
			t.Fatalf("%s missing: %v", want, err)
		}
	}
}

func TestKovaloopLedgerSkillDescriptionRoutesCommonIntents(t *testing.T) {
	skill := readRepoFile(t, "skills", "kovaloop-ledger", "SKILL.md")
	description := frontmatterDescription(t, skill)

	for _, want := range []string{
		"claimCode",
		"claim link",
		"领取钱包",
		"查余额",
		"到账了吗",
		"转账",
		"USDC",
		"route payment",
		"Do NOT use for ledger credit",
		"Do NOT use for direct Circle",
		"external agent",
	} {
		if !strings.Contains(description, want) {
			t.Fatalf("description missing routing phrase %q\n%s", want, description)
		}
	}
}

func frontmatterDescription(t *testing.T, skill string) string {
	t.Helper()
	lines := strings.Split(skill, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		t.Fatal("missing frontmatter")
	}

	var out []string
	inDescription := false
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			break
		}
		if strings.HasPrefix(line, "description:") {
			inDescription = true
			out = append(out, strings.TrimSpace(strings.TrimPrefix(line, "description:")))
			continue
		}
		if inDescription {
			if strings.HasPrefix(line, "  ") || strings.TrimSpace(line) == "" {
				out = append(out, strings.TrimSpace(line))
				continue
			}
			break
		}
	}
	return strings.Join(out, "\n")
}
