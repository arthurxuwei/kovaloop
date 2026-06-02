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

func TestKovaloopLedgerSkillDocumentsPrivateRiskControls(t *testing.T) {
	skill := readRepoFile(t, "skills", "kovaloop-ledger", "SKILL.md")
	routing := readRepoFile(t, "skills", "kovaloop-ledger", "references", "payment-routing.md")
	directTransfer := readRepoFile(t, "skills", "kovaloop-ledger", "references", "direct-transfer.md")
	combined := skill + "\n" + routing + "\n" + directTransfer

	for _, want := range []string{"private risk controls", "do not perform local risk or limit checks", "Do not guess, disclose, or explain concrete thresholds"} {
		if !strings.Contains(combined, want) {
			t.Fatalf("service limit docs missing %q", want)
		}
	}
	for _, forbidden := range []string{"0.001 USDC", "5 USDC", "10 USDC", "rolling 24", "rolling 7"} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("skill docs disclose risk threshold %q", forbidden)
		}
	}
}

func TestKovaloopLedgerSkillPreservesAtomicUSDCPrecision(t *testing.T) {
	skill := readRepoFile(t, "skills", "kovaloop-ledger", "SKILL.md")
	balanceState := readRepoFile(t, "skills", "kovaloop-ledger", "references", "balance-state.md")
	combined := skill + "\n" + balanceState

	for _, want := range []string{
		"amountDisplay",
		"availableDeltaDisplay",
		"1 USDC = 1000000 atomic units",
		"0.000001 USDC",
		"Never report a non-zero atomic amount as `0 USDC`",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("skill docs missing atomic precision rule %q", want)
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
