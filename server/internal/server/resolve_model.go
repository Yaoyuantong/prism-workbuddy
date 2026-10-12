package server

import "strings"

// resolveModel 解析模型名协议（PLAN D6）：
//
//	分布式前缀： "[realm:]model"
//
// 取第一个 ":"，前段恰为 "cn"/"global" 才剥离；否则视为裸名，realm=cn、bare=原串。
// 大小写敏感（前缀必须是精确的小写枚举）。bare 即出站/选号/账本使用的裸模型名。
//
// 导出为 ResolveModel（cmd/server/main.go 粘性闭包需要），包内简写 resolveModel。
func resolveModel(model string) (realm, bare string) {
	idx := strings.IndexByte(model, ':')
	if idx < 0 {
		return "cn", model
	}
	prefix := model[:idx]
	if prefix != "cn" && prefix != "global" {
		return "cn", model
	}
	return prefix, model[idx+1:]
}

// ResolveModel 是 resolveModel 的导出面（跨包调用）。
func ResolveModel(model string) (realm, bare string) { return resolveModel(model) }

// applyModelAlias 按别名表改写裸模型名（G-ALIAS）：
//
//	aliases["客户端发的裸名"] = "上游真名"
//
// 未命中 / 表为空 / 命中值为空串 → 原样返回。大小写敏感（模型 ID 是精确串）。
//
// 为什么单列成纯函数：这是「客户端发的名字」与「上游真名」之间**唯一**的换算点，
// 单测可以直接覆盖全部边界，不必搭 Handler/上游。
//
// 为什么在 resolveModel **之后**应用：realm 前缀是网关侧路由协议（cn:/global:
// 决定走哪个域的账号），别名只管裸名。先剥前缀再查表，则下游所有环节
//（per-model 限流、选号可用性、模型级冷却、用量账本、出站 body 重写）看到的
// 都是同一个真名 —— 不会出现「别名一套账、真名一套账」。
func applyModelAlias(aliases map[string]string, bare string) string {
	if len(aliases) == 0 || bare == "" {
		return bare
	}
	if to, ok := aliases[bare]; ok && to != "" {
		return to
	}
	return bare
}
