package server

import "testing"

// TestApplyModelAlias 覆盖别名改写的全部边界（G-ALIAS 的唯一换算点）。
func TestApplyModelAlias(t *testing.T) {
	cases := []struct {
		name    string
		aliases map[string]string
		in      string
		want    string
	}{
		{"nil 表原样", nil, "deepseek-v4.1-flash", "deepseek-v4.1-flash"},
		{"空表原样", map[string]string{}, "m", "m"},
		{"空串入参原样", map[string]string{"": "x"}, "", ""},
		{"命中改写", map[string]string{"old-name": "new-name"}, "old-name", "new-name"},
		{"未命中原样", map[string]string{"old-name": "new-name"}, "other", "other"},
		{"命中但空值不替换", map[string]string{"old-name": ""}, "old-name", "old-name"},
		{"大小写敏感（不命中）", map[string]string{"Old": "New"}, "old", "old"},
		{"多键只取命中项", map[string]string{"a": "x", "b": "y"}, "b", "y"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := applyModelAlias(tc.aliases, tc.in); got != tc.want {
				t.Fatalf("applyModelAlias(%v, %q)=%q want %q", tc.aliases, tc.in, got, tc.want)
			}
		})
	}
}

// TestApplyModelAliasAfterResolve 锁定调用顺序契约：别名在 resolveModel **之后**应用，
// 即 realm 前缀（cn:/global:）先被剥掉，别名只映射裸模型名。
//
// 这正是「下游全环节看到同一个真名」的前提：若别名作用在剥前缀之前，
// "cn:old" 与 "old" 会走两套账（per-model 限流 / 选号可用性 / 模型级冷却 / 账本）。
func TestApplyModelAliasAfterResolve(t *testing.T) {
	aliases := map[string]string{"old-name": "new-name"}

	// cn: 前缀被剥离后命中别名。
	realm, bare := resolveModel("cn:old-name")
	if realm != "cn" || bare != "old-name" {
		t.Fatalf("resolveModel: realm=%q bare=%q", realm, bare)
	}
	if got := applyModelAlias(aliases, bare); got != "new-name" {
		t.Fatalf("cn:old-name should alias to new-name, got %q", got)
	}

	// global: 前缀同理。
	realm, bare = resolveModel("global:old-name")
	if realm != "global" || bare != "old-name" {
		t.Fatalf("resolveModel: realm=%q bare=%q", realm, bare)
	}
	if got := applyModelAlias(aliases, bare); got != "new-name" {
		t.Fatalf("global:old-name should alias to new-name, got %q", got)
	}

	// 别名表里的键是"裸名"：带前缀的写法不会被命中（因为查表前前缀已剥离）。
	// 说明用户应配裸名，而不是 cn:old-name。
	if got := applyModelAlias(map[string]string{"cn:old-name": "x"}, "old-name"); got != "old-name" {
		t.Fatalf("带前缀的别名键不应命中裸名，got %q", got)
	}

	// 无前缀裸名直接命中。
	if _, bare := resolveModel("old-name"); applyModelAlias(aliases, bare) != "new-name" {
		t.Fatalf("bare old-name should alias to new-name")
	}
}

// TestApplyModelAliasIdempotent 别名指向的名字再次查表不会二次改写
// （表里没有 new-name 键）——避免链式改写这种意料外的语义。
func TestApplyModelAliasIdempotent(t *testing.T) {
	aliases := map[string]string{"old-name": "new-name"}
	once := applyModelAlias(aliases, "old-name")
	twice := applyModelAlias(aliases, once)
	if once != "new-name" || twice != "new-name" {
		t.Fatalf("once=%q twice=%q want both new-name", once, twice)
	}
}
