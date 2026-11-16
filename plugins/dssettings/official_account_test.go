package dssettings

import (
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

func accountString(value string) *string { return &value }

func TestFormatAccountBalanceOfficialRules(t *testing.T) {
	cases := []struct {
		amount string
		want   string
	}{
		{"0", "¥0.00"},
		{"0.009", "<¥0.01"},
		{"-0.009", "-¥0.01"},
		{"1234567.899", "¥1,234,567.89"},
		{"-1234.567", "-¥1,234.57"},
		{"0.01", "¥0.01"},
		{"1.005", "¥1.00"},
	}
	for _, tc := range cases {
		got, ok := formatAccountBalance(tc.amount, "¥")
		if !ok || got != tc.want {
			t.Fatalf("formatAccountBalance(%q) = %q, %v；want %q", tc.amount, got, ok, tc.want)
		}
	}
	for _, invalid := range []string{"", "abc", "1/3"} {
		if got, ok := formatAccountBalance(invalid, "¥"); ok {
			t.Fatalf("非法金额 %q 被格式化成了 %q", invalid, got)
		}
	}
}

func TestAccountBonusRowHiddenWhenReadyAndAllZero(t *testing.T) {
	snapshot := apiclient.AccountSnapshot{Balance: &apiclient.AccountBalance{
		Status: "ready", BonusWallets: []apiclient.AccountWallet{{Currency: "CNY", Balance: "0.00"}},
	}}
	if accountBonusVisible(snapshot) {
		t.Fatal("ready 且赠金全 0 时官方隐藏赠金行")
	}
	snapshot.Balance.BonusWallets[0].Balance = "0.01"
	if !accountBonusVisible(snapshot) {
		t.Fatal("赠金大于 0 时必须显示")
	}
	snapshot.Balance.BonusWallets[0].Balance = "not-a-decimal"
	if !accountBonusVisible(snapshot) {
		t.Fatal("非法赠金必须显示不可用，不能静默隐藏")
	}
}

func pumpAccountUntil(t *testing.T, h *officialHarness, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		h.pump()
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("账号页条件未在 3s 内成立；texts=%v", h.session.Texts())
}

func TestAccountPageRendersRealProfileAndBalances(t *testing.T) {
	h := openOfficialHarness(t, 1280, 800)
	defer h.close()
	h.mu.Lock()
	h.account = apiclient.AccountSnapshot{
		View: &apiclient.AccountView{
			Status: apiclient.AccountCredentialStored,
			Links:  apiclient.AccountLinks{UsageURL: "https://platform.example/usage", TopUpURL: "https://platform.example/top-up"},
		},
		Known: true,
		Profile: &apiclient.AccountProfile{Status: "ready", Value: &apiclient.AccountProfileValue{
			Name: accountString("真实用户"), Contact: accountString("real@example.com"),
		}},
		Balance: &apiclient.AccountBalance{Status: "ready",
			Value:        []apiclient.AccountWallet{{Currency: "CNY", Balance: "1234.567"}},
			BonusWallets: []apiclient.AccountWallet{{Currency: "USD", Balance: "0.25"}},
		},
	}
	h.mu.Unlock()
	h.p.SelectSection("account")
	if !h.p.OpenOfficial() {
		t.Fatal("打开设置失败")
	}
	pumpAccountUntil(t, h, func() bool {
		return h.hasText("真实用户") && h.hasText("real@example.com") && h.hasText("¥1,234.56") && h.hasText("$0.25")
	})
	h.mu.Lock()
	refreshes := h.accountRefresh
	h.mu.Unlock()
	if refreshes != 1 {
		t.Fatalf("设置 false→true 边沿应只刷新一次，实际 %d", refreshes)
	}
	h.click("account-usage")
	pumpAccountUntil(t, h, func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		return len(h.accountExternals) == 1 && h.accountExternals[0] == "https://platform.example/usage"
	})
	h.click("account-top-up")
	pumpAccountUntil(t, h, func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		return len(h.accountExternals) == 2 && h.accountExternals[1] == "https://platform.example/top-up"
	})
}

func TestAccountPageCancelUsesRealAttemptID(t *testing.T) {
	h := openOfficialHarness(t, 1280, 800)
	defer h.close()
	h.mu.Lock()
	h.account.View = &apiclient.AccountView{
		Status:  apiclient.AccountCredentialStored,
		Attempt: &apiclient.AccountAttempt{ID: "attempt-real", Phase: "waiting-browser", AuthorizeURL: "https://platform.example/auth"},
	}
	h.mu.Unlock()
	h.p.SelectSection("account")
	if !h.p.OpenOfficial() {
		t.Fatal("打开设置失败")
	}
	pumpAccountUntil(t, h, func() bool { return h.hasText("请在浏览器中继续") })
	h.click("account-cancel-sign-in")
	pumpAccountUntil(t, h, func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		return len(h.accountCancels) == 1 && h.accountCancels[0] == "attempt-real"
	})
}
