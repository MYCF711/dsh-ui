package dssettings

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
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

// TestAccountUnavailableBalanceRowIsPlatformLink：官方 AccountSection.tsx:186-188 /
// 200-201 的第三分支——余额有结果但没有钱包（读失败）时，那一行不是静默灰字，
// 而是指向 `state.links.usageUrl` 的 `.unavailableLink`
// （AccountSection.module.css:123-141：下划线 + offset 3px）。
func TestAccountUnavailableBalanceRowIsPlatformLink(t *testing.T) {
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
			Name: accountString("真实用户"),
		}},
		// 读到结果但钱包为空 = 读失败（官方 details.balance.status !== 'ready'）。
		Balance: &apiclient.AccountBalance{Status: "failed"},
	}
	h.mu.Unlock()
	h.p.SelectSection("account")
	if !h.p.OpenOfficial() {
		t.Fatal("打开设置失败")
	}
	pumpAccountUntil(t, h, func() bool { return h.hasText("前往开放平台查看") })
	h.click("account-balance-unavailable")
	pumpAccountUntil(t, h, func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		return len(h.accountExternals) == 1 && h.accountExternals[0] == "https://platform.example/usage"
	})
}

// TestAccountBalanceSignedOutCopy：官方 `!signedIn` 分支是 `balanceSignedOut`
// （locales.ts:66「登录后查看」），不是「加载中…」也不是平台链接。
func TestAccountBalanceSignedOutCopy(t *testing.T) {
	h := openOfficialHarness(t, 1280, 800)
	defer h.close()
	h.mu.Lock()
	h.account = apiclient.AccountSnapshot{
		View:  &apiclient.AccountView{Status: "signed-out", Links: apiclient.AccountLinks{UsageURL: "https://platform.example/usage"}},
		Known: true,
	}
	h.mu.Unlock()
	h.p.SelectSection("account")
	if !h.p.OpenOfficial() {
		t.Fatal("打开设置失败")
	}
	pumpAccountUntil(t, h, func() bool { return h.hasText("当前未登录 DeepSeek Harness 账号") })
	if h.hasText("前往开放平台查看") {
		t.Fatal("未登录时不能画平台链接（官方那时是 balanceSignedOut 灰字）")
	}
}

// TestAccountAvatarOnlyDrawsReadyBitmap：官方 AccountAvatar.tsx:15-18——只有
// status==='ready' 才画 `<img>`，否则画 `IconUserOutlineMedium size={16}`。
// 这里用真实 PNG 走完整"下载 + 解码 + 发布"链路（loader 见 official_account_avatar.go）。
func TestAccountAvatarOnlyDrawsReadyBitmap(t *testing.T) {
	png := makeAccountAvatarPNG(t, 3, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Referer") != "" {
			t.Errorf("官方 referrerPolicy='no-referrer'：请求不能带 Referer，实际 %q", request.Header.Get("Referer"))
		}
		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write(png)
	}))
	defer server.Close()

	var events []AccountAvatarStatus
	loader := &accountAvatarLoader{
		client:  server.Client(),
		publish: func(snapshot AccountAvatarSnapshot) { events = append(events, snapshot.Status) },
	}
	url := server.URL + "/avatar.png"
	if !loader.setURL(url) {
		t.Fatal("首次 setURL 必须发布一次")
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if snapshot := loader.snapshotValue(); snapshot != nil && snapshot.Status == AccountAvatarReady {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	snapshot := loader.snapshotValue()
	if snapshot == nil || snapshot.Status != AccountAvatarReady {
		t.Fatalf("真实 PNG 必须走完到 ready：%+v", snapshot)
	}
	if snapshot.Bitmap.Width != 3 || snapshot.Bitmap.Height != 2 || snapshot.Bitmap.Stride != 12 || len(snapshot.Bitmap.Pix) != 24 {
		t.Fatalf("位图几何错：%+v", snapshot.Bitmap)
	}
	// 官方先发 loading 再发 ready（account-avatar.ts:75 → :73）。
	if len(events) < 2 || events[0] != AccountAvatarLoading || events[len(events)-1] != AccountAvatarReady {
		t.Fatalf("发布序列必须是 loading→ready：%v", events)
	}
	// 同 URL 不重复下载（account-avatar.ts:27）。
	if loader.setURL(url) {
		t.Fatal("同 URL 的 setURL 必须是空操作")
	}
	// 空 URL 清空（account-avatar.ts:28-31）。
	if !loader.setURL("") || loader.snapshotValue() != nil {
		t.Fatal("空 URL 必须清空快照")
	}
	loader.dispose()
}

// TestAccountAvatarFailedImageFallsBackToIcon：非 2xx / 非 http(s) 一律 failed，
// 界面因此画官方兜底图标而不是空白（AccountAvatar.tsx:18）。
func TestAccountAvatarFailedImageFallsBackToIcon(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	loader := &accountAvatarLoader{client: server.Client()}
	loader.setURL(server.URL + "/blocked.png")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if snapshot := loader.snapshotValue(); snapshot != nil && snapshot.Status == AccountAvatarFailed {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if snapshot := loader.snapshotValue(); snapshot == nil || snapshot.Status != AccountAvatarFailed {
		t.Fatalf("403 必须落到 failed：%+v", snapshot)
	}
	if loader.retry() != true {
		t.Fatal("failed 时 retry 必须重启一次加载")
	}
	loader.dispose()
	if loader.setURL("https://example.invalid/a.png") {
		t.Fatal("dispose 之后 setURL 必须被丢弃（account-avatar.ts:25）")
	}

	offline := &accountAvatarLoader{}
	offline.setURL("file:///C:/secret.png")
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if snapshot := offline.snapshotValue(); snapshot != nil && snapshot.Status == AccountAvatarFailed {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("非 http(s) 必须 failed——不许读本地磁盘")
}

// makeAccountAvatarPNG 造一张真 PNG（走标准库编码，测试不引入新依赖）。
func makeAccountAvatarPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			canvas.Set(x, y, color.RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, canvas); err != nil {
		t.Fatalf("造 PNG 失败：%v", err)
	}
	return buffer.Bytes()
}
