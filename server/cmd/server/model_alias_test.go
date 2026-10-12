package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/linguo2625469/workbuddy2api-panel/internal/livecfg"
	"github.com/linguo2625469/workbuddy2api-panel/internal/pool"
	"github.com/linguo2625469/workbuddy2api-panel/internal/scheduler"
	"github.com/linguo2625469/workbuddy2api-panel/internal/upstream"
)

// TestNormalizeModelAlias 覆盖 model_alias 的校验与归一（G-ALIAS）。
func TestNormalizeModelAlias(t *testing.T) {
	// 空表 / 缺省 → nil（= 不启用，零回归）。
	for _, body := range []string{`{}`, `{"model_alias":{}}`, `{"model_alias":null}`} {
		c, err := ParseConfig([]byte(body))
		if err != nil {
			t.Fatalf("ParseConfig(%s): %v", body, err)
		}
		if c.ModelAlias != nil {
			t.Errorf("%s: ModelAlias=%v want nil", body, c.ModelAlias)
		}
	}

	// 正常映射保留；两端空白被裁掉。
	c, err := ParseConfig([]byte(`{"model_alias":{"  deepseek-v4.1-flash  ":"  deepseek-v4-flash  "}}`))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if len(c.ModelAlias) != 1 || c.ModelAlias["deepseek-v4.1-flash"] != "deepseek-v4-flash" {
		t.Fatalf("ModelAlias=%v want trimmed map", c.ModelAlias)
	}

	// 自映射（k==v）等于没配 → 丢弃；全丢弃后归一为 nil。
	c, err = ParseConfig([]byte(`{"model_alias":{"same":"same"}}`))
	if err != nil {
		t.Fatalf("ParseConfig self-map: %v", err)
	}
	if c.ModelAlias != nil {
		t.Fatalf("self-map should be dropped -> nil, got %v", c.ModelAlias)
	}
	// 自映射与有效项混排：只丢自映射那一条。
	c, err = ParseConfig([]byte(`{"model_alias":{"same":"same","a":"b"}}`))
	if err != nil {
		t.Fatalf("ParseConfig mixed: %v", err)
	}
	if len(c.ModelAlias) != 1 || c.ModelAlias["a"] != "b" {
		t.Fatalf("mixed map=%v want {a:b}", c.ModelAlias)
	}
}

// TestNormalizeModelAliasRejects 非法项 fail fast，且错误信息点名具体那条映射
// （面板会原样透出——静默丢弃会让用户以为"保存成功"却毫无效果）。
func TestNormalizeModelAliasRejects(t *testing.T) {
	cases := []struct{ body, wantSub string }{
		{`{"model_alias":{"a":""}}`, "空串"},
		{`{"model_alias":{"":"b"}}`, "空串"},
		{`{"model_alias":{"   ":"b"}}`, "空串"},
		{`{"model_alias":{"cn:a":"b"}}`, "realm"},
		{`{"model_alias":{"a":"global:b"}}`, "realm"},
	}
	for _, tc := range cases {
		if _, err := ParseConfig([]byte(tc.body)); err == nil {
			t.Errorf("%s: want error", tc.body)
		} else if !strings.Contains(err.Error(), tc.wantSub) {
			t.Errorf("%s: error %q should contain %q", tc.body, err.Error(), tc.wantSub)
		}
	}
}

// TestLoadModelAliasEndToEnd 从 config 文件加载别名并生效（含未知键不影响）。
func TestLoadModelAliasEndToEnd(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "c.json")
	os.WriteFile(fp, []byte(`{"listen":":9999","model_alias":{"old":"new"},"手写未知键":123}`), 0o600)
	c, err := Load(fp)
	if err != nil {
		t.Fatal(err)
	}
	if c.ModelAlias["old"] != "new" || len(c.ModelAlias) != 1 {
		t.Fatalf("ModelAlias=%v want {old:new}", c.ModelAlias)
	}
	// 老 config（无 model_alias 段）零影响。
	fp2 := filepath.Join(dir, "c2.json")
	os.WriteFile(fp2, []byte(`{"listen":":9999"}`), 0o600)
	c2, err := Load(fp2)
	if err != nil {
		t.Fatal(err)
	}
	if c2.ModelAlias != nil {
		t.Fatalf("legacy config ModelAlias=%v want nil", c2.ModelAlias)
	}
}

// TestMergeConfigMapsModelAliasWholeReplace 锁定 replaceWholeKeys 语义：
// model_alias 是「整表替换」，而普通嵌套表仍是「深合并」。
func TestMergeConfigMapsModelAliasWholeReplace(t *testing.T) {
	cur := map[string]any{
		"model_alias": map[string]any{"a": "b", "c": "d"},
		"pool":        map[string]any{"max_in_flight": float64(3), "breaker_threshold": float64(3)},
	}
	incoming := map[string]any{
		// 用户删掉了 a：incoming 只剩 c。
		"model_alias": map[string]any{"c": "d"},
		// 面板只提交它管的键：未提交的兄弟键（breaker_threshold）必须保留。
		"pool": map[string]any{"max_in_flight": float64(5)},
	}
	mergeConfigMaps(cur, incoming)

	alias, _ := cur["model_alias"].(map[string]any)
	if len(alias) != 1 || alias["c"] != "d" {
		t.Fatalf("model_alias should be WHOLE-replaced (a must be gone), got %v", alias)
	}
	p, _ := cur["pool"].(map[string]any)
	if p["max_in_flight"] != float64(5) || p["breaker_threshold"] != float64(3) {
		t.Fatalf("pool should be DEEP-merged (sibling kept), got %v", p)
	}
}

// TestSaveConfigModelAliasHotApply 端到端验证「面板保存 alias → 落盘 + 热生效」：
// 删掉一条后，磁盘与新快照都只应剩提交的那条。
func TestSaveConfigModelAliasHotApply(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "config.json")
	os.WriteFile(fp, []byte(`{"model_alias":{"a":"b","c":"d"}}`), 0o600)

	p := pool.New("")
	up := upstream.New()
	sch := scheduler.New(scheduler.Config{Pool: p, Upstream: up})
	live := livecfg.New(livecfg.Snapshot{})
	// 初始快照带旧别名，模拟进程启动时已加载。
	live.Store(livecfg.Snapshot{ModelAliases: map[string]string{"a": "b", "c": "d"}})

	// 面板提交「只剩 c」。
	if _, err := saveConfig([]byte(`{"model_alias":{"c":"d"}}`), fp, live, p, up, sch); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}

	// 磁盘：a 必须消失（整表替换）。
	raw, err := os.ReadFile(fp)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk struct {
		ModelAlias map[string]string `json:"model_alias"`
	}
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("unmarshal saved config: %v (%s)", err, raw)
	}
	if len(onDisk.ModelAlias) != 1 || onDisk.ModelAlias["c"] != "d" {
		t.Fatalf("on-disk model_alias=%v want {c:d} (a must be deleted)", onDisk.ModelAlias)
	}

	// 热快照：后续请求立即看到新表。
	if got := live.Load().ModelAliases; len(got) != 1 || got["c"] != "d" {
		t.Fatalf("live snapshot ModelAliases=%v want {c:d}", got)
	}
}

// TestSaveConfigModelAliasRejectsBadInput 面板提交非法映射 → 报错且不落盘。
func TestSaveConfigModelAliasRejectsBadInput(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "config.json")
	before := `{"model_alias":{"a":"b"}}`
	os.WriteFile(fp, []byte(before), 0o600)

	p := pool.New("")
	up := upstream.New()
	sch := scheduler.New(scheduler.Config{Pool: p, Upstream: up})
	live := livecfg.New(livecfg.Snapshot{})

	if _, err := saveConfig([]byte(`{"model_alias":{"x":"cn:y"}}`), fp, live, p, up, sch); err == nil {
		t.Fatal("want error for realm-prefixed alias")
	}
	// 落盘内容未被动过。
	raw, _ := os.ReadFile(fp)
	if string(raw) != before {
		t.Fatalf("config file should be untouched on validation failure, got %s", raw)
	}
}
