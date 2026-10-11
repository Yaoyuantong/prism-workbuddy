package pool

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/linguo2625469/workbuddy2api-panel/internal/auth"
)

// TestPinnedUIDPrefersPinned 钉号优先：pinned_uid 指向的账号即使权重最低（credits=0）
// 也应在每次 Pick 中被选中。
func TestPinnedUIDPrefersPinned(t *testing.T) {
	withNoPickGap(t)
	p := New("")
	for _, u := range []string{"u1", "u2", "u3"} {
		p.Add(&auth.Auth{UID: u})
	}
	p.SetCredits("u1", 50000, 0)
	p.SetCredits("u3", 50000, 0)
	p.SetCredits("u2", 0, 0) // 权重最低：不钉它几乎不可能被选中
	p.SetPinnedUID("u2")

	for i := 0; i < 300; i++ {
		got := p.Pick()
		if got == nil || got.UID != "u2" {
			t.Fatalf("pick #%d = %v, want pinned u2", i, got)
		}
	}
	if got := p.PinnedUID(); got != "u2" {
		t.Errorf("PinnedUID()=%q want u2", got)
	}
}

// TestPinnedUIDFallsBackWhenDisabled 钉号优雅降级：pinned 账号被 disabled 时必须
// 回落常规轮换（返回其他健康号），不得卡死/返回错误。
func TestPinnedUIDFallsBackWhenDisabled(t *testing.T) {
	withNoPickGap(t)
	p := New("")
	p.Add(&auth.Auth{UID: "u1"})
	p.Add(&auth.Auth{UID: "u2"})
	p.SetPinnedUID("u2")
	p.Disable("u2", "test")

	seen := map[string]int{}
	for i := 0; i < 200; i++ {
		got := p.Pick()
		if got == nil {
			t.Fatal("pick returned nil: pinned disabled should fall back to u1")
		}
		if got.UID == "u2" {
			t.Fatal("disabled pinned account must not be picked")
		}
		seen[got.UID]++
	}
	if seen["u1"] == 0 {
		t.Errorf("expected fallback to u1, seen=%v", seen)
	}
}

// TestPinnedUIDFallsBackWhenCooling 钉号优雅降级：pinned 账号冷却中 → 回落。
func TestPinnedUIDFallsBackWhenCooling(t *testing.T) {
	withNoPickGap(t)
	p := New("")
	p.Add(&auth.Auth{UID: "u1"})
	p.Add(&auth.Auth{UID: "u2"})
	p.SetPinnedUID("u2")
	p.Cooldown("u2", CoolSoft, time.Hour, "429 rate limit")

	for i := 0; i < 100; i++ {
		got := p.Pick()
		if got == nil || got.UID != "u1" {
			t.Fatalf("cooling pinned account must fall back to u1; got %v", got)
		}
	}
}

// TestPinnedUIDFallsBackWhenPaused 钉号优雅降级：pinned 账号暂停选号 → 回落。
func TestPinnedUIDFallsBackWhenPaused(t *testing.T) {
	withNoPickGap(t)
	p := New("")
	p.Add(&auth.Auth{UID: "u1"})
	p.Add(&auth.Auth{UID: "u2"})
	p.SetPinnedUID("u2")
	p.Pause("u2")

	for i := 0; i < 100; i++ {
		got := p.Pick()
		if got == nil || got.UID != "u1" {
			t.Fatalf("paused pinned account must fall back to u1; got %v", got)
		}
	}
}

// TestPinnedUIDClearedByEmpty 空串取消钉号：清空后选号回到常规加权（不再是恒 u2）。
func TestPinnedUIDClearedByEmpty(t *testing.T) {
	withNoPickGap(t)
	p := New("")
	p.Add(&auth.Auth{UID: "u1"})
	p.Add(&auth.Auth{UID: "u2"})
	p.SetPinnedUID("u2")
	p.SetPinnedUID("")

	seen := map[string]int{}
	for i := 0; i < 300; i++ {
		got := p.Pick()
		if got == nil {
			t.Fatal("nil pick")
		}
		seen[got.UID]++
	}
	if seen["u1"] == 0 || seen["u2"] == 0 {
		t.Errorf("after clearing pin both accounts should appear; seen=%v", seen)
	}
}

// TestPinnedUIDRespectsTried 请求级轮换（tried）优先于钉号：pinned 号被 tried 排除时
// 不得返回它（避免同一请求轮换时反复砸同一个号）。
func TestPinnedUIDRespectsTried(t *testing.T) {
	withNoPickGap(t)
	p := New("")
	p.Add(&auth.Auth{UID: "u1"})
	p.Add(&auth.Auth{UID: "u2"})
	p.SetPinnedUID("u2")

	got := p.PickExcluding(map[string]bool{"u2": true})
	if got == nil || got.UID != "u1" {
		t.Fatalf("tried pinned account must be excluded; got %v", got)
	}
}

// TestClaimedCreditTodayAndTotal 今日已领积分累计与全池求和；n<=0 为空操作。
func TestClaimedCreditTodayAndTotal(t *testing.T) {
	p := New("")
	p.Add(&auth.Auth{UID: "u1"})
	p.Add(&auth.Auth{UID: "u2"})

	p.AddClaimedCredit("u1", 100)
	p.AddClaimedCredit("u1", 50)
	p.AddClaimedCredit("u2", 30)
	p.AddClaimedCredit("u1", 0)   // 空操作
	p.AddClaimedCredit("u1", -5)  // 空操作
	p.AddClaimedCredit("nope", 9) // 不存在的 uid：忽略

	st, _ := p.Status("u1")
	if st.ClaimedTodayCredit != 150 {
		t.Errorf("u1 claimed_today_credit=%d want 150", st.ClaimedTodayCredit)
	}
	st2, _ := p.Status("u2")
	if st2.ClaimedTodayCredit != 30 {
		t.Errorf("u2 claimed_today_credit=%d want 30", st2.ClaimedTodayCredit)
	}
	if sum := p.ClaimedTodayTotal(); sum != 180 {
		t.Errorf("ClaimedTodayTotal()=%d want 180", sum)
	}
}

// TestClaimedCreditDayRollover 跨天归零：claimedDay 非当日时读取侧返回 0，写入时先清零。
func TestClaimedCreditDayRollover(t *testing.T) {
	p := New("")
	p.Add(&auth.Auth{UID: "u1"})
	p.AddClaimedCredit("u1", 200)
	if sum := p.ClaimedTodayTotal(); sum != 200 {
		t.Fatalf("precondition: total=%d want 200", sum)
	}
	// 人为把日期回退到昨天（模拟隔夜未清零的残留）。
	p.mu.Lock()
	p.byUID["u1"].claimedDay = "2000-01-01"
	p.mu.Unlock()

	if st, _ := p.Status("u1"); st.ClaimedTodayCredit != 0 {
		t.Errorf("stale day must read 0, got %d", st.ClaimedTodayCredit)
	}
	if sum := p.ClaimedTodayTotal(); sum != 0 {
		t.Errorf("stale day total must be 0, got %d", sum)
	}
	// 隔天首次写入：先清零再加新值（不叠加隔夜残留）。
	p.AddClaimedCredit("u1", 40)
	if sum := p.ClaimedTodayTotal(); sum != 40 {
		t.Errorf("rollover should reset then add: total=%d want 40", sum)
	}
}

// TestClaimedCreditPersists 今日已领积分随 state.json 持久化，重启后仍在。
func TestClaimedCreditPersists(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "state.json")
	p := New(fp)
	p.Add(&auth.Auth{UID: "u1"})
	p.AddClaimedCredit("u1", 123)
	p.Flush() // dirty → 落盘

	p2 := New(fp)
	p2.Add(&auth.Auth{UID: "u1"})
	if st, _ := p2.Status("u1"); st.ClaimedTodayCredit != 123 {
		t.Errorf("claimed_today_credit lost after reload: %+v", st)
	}
	if sum := p2.ClaimedTodayTotal(); sum != 123 {
		t.Errorf("total after reload=%d want 123", sum)
	}
}

// TestPinnedUIDPersistsInOverviewShape 钉号经 SetPinnedUID 生效并可读回（overview 透出用）。
func TestPinnedUIDPersistsInOverviewShape(t *testing.T) {
	p := New("")
	p.Add(&auth.Auth{UID: "u1"})
	if got := p.PinnedUID(); got != "" {
		t.Errorf("default PinnedUID()=%q want empty", got)
	}
	p.SetPinnedUID("  u1  ") // 前后空白应被裁剪
	if got := p.PinnedUID(); got != "u1" {
		t.Errorf("PinnedUID()=%q want u1 (trimmed)", got)
	}
}
