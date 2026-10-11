# CHANGES-vs-upstream

本仓库是对上游开源项目 **`linguo2625469/workbuddy2api-panel` v1.13.0** 的 fork。
本文档逐文件说明**我们相对上游 v1.13.0 做了什么、为什么**，作为维护者/审查者的对照底稿，
也可作为向上游提交 PR 的素材。

- 上游版本：v1.13.0（源码树 167 文件）
- 我们的源码树：`oss/server/`（168 文件 = 上游 167 + 1 个新增测试文件）
- 改动范围：**19 个文件**（18 个修改 + 1 个新增），其余 149 个文件与上游逐字节一致。

## 改动概述

两条产品需求，基座原生没有：

1. **钉号（主号）** —— 让用户指定「网关固定走某个账号」。上游只有 `paused`/`disabled`，
   没有 pin/priority 概念。只做 UI 层假标记是骗人的（界面显示主号、实际轮转照旧），
   因此真的改了选号逻辑。
2. **今日已领积分** —— 上游账号模型只有 `checkin_done` 布尔，没有「今天领了多少积分」。

契约字段（供前端/适配层对接）：
- `GET /panel/api/overview` 顶层新增 `pinned_uid`(string, ""=未钉)、`claimed_today`(int64, 各账号求和)
- `accounts[]` 每项新增 `claimed_today_credit`(int64, 0=未知/今日未领)
- 配置 `pool.pinned_uid`(string)，`POST /panel/api/config` 改完**立即热生效**

---

## cmd/server/config.go
- 新增 `Pool.PinnedUID string \`json:"pinned_uid"\``（默认 `""` = 不钉）。
- 原因：产品需要「钉号/主号」配置项；上游 `pool` 段无此字段。

## cmd/server/main.go
- 启动装配：`p.SetPinnedUID(cfg.Pool.PinnedUID)`。
- `saveConfig` 热应用：`p.SetPinnedUID(newCfg.Pool.PinnedUID)`。
- 同步更新 `saveConfig` 顶部热生效字段清单注释。
- 原因：钉号必须「改配置即生效」，走基座已有的 livecfg/热应用机制（与 `SetPreferExpiring` 等同列）。

## internal/pool/pool.go
- 新增字段 `Pool.pinnedUID string`。
- 新增 `SetPinnedUID(uid string)`（`strings.TrimSpace` 后写入）与 `PinnedUID() string`（RLock 读取）。
- 原因：钉号需要并发安全的读写点，且要能被 `pick` 热路径读取、被 `overview` 透出。

## internal/pool/pick.go
- 在选号入口 `pick()` 持写锁后、展开候选集之前，新增**钉号快路径**：
  当 `pinnedUID` 非空、且该账号在当前上下文全部通过时，直接返回它。
- **降级规则（重要，务必保持）**：钉号命中需同时满足——
  1. 未被 `tried` 排除（请求级轮换优先于钉号）；
  2. `healthyOf(pe)` 为真（账号级健康：非 disabled、非 paused，且软/硬冷却、熔断、连败降权
     的截止时间全部已过）→ 覆盖 **disabled / paused / 冷却中 / 熔断中 / 连败降权**；
  3. 未触发积分保底 `floorBlocked(pe)`（触底账号不接收费模型）→ 覆盖 **积分耗尽（触底+收费）**；
  4. 未占满在途名额 `inFlightFull(pe)`。
  上述**任一不满足即静默回落到原有加权逻辑**，绝不卡死、不返回错误。
  此外，「余额耗尽」在上游表现为 `CoolHard` 冷却到次日 04:00（`until` 未过期 → `healthy=false`），
  也由第 2 条同一路径降级。
- 刻意让钉号**绕过 `minPickGap` 防撞号窗口**：钉号的语义就是「固定走它」，若被该窗口挤开
  就退化成普通轮换，与用户预期不符。
- 原因：需求 1。改动零侵入——不满足条件时完全走原算法，上游行为不受影响。

## internal/pool/entry.go
- 运行态 `entry` 新增 `claimedCredit int64` + `claimedDay string`，及方法
  `claimedCreditToday(now) int64`（`claimedDay` 非当日时返回 0 —— 跨天读侧归零）。
- `Status` 新增 `ClaimedTodayCredit int64 \`json:"claimed_today_credit"\``（**恒输出**，0 是有效语义）。
- `stateAccount` 新增 `ClaimedTodayCredit int64 \`json:"claimed_today_credit,omitempty"\``
  与 `ClaimedDay string \`json:"claimed_day,omitempty"\``。
- 原因：需求 2 的字段载体 + 持久化模型。⚠️ 新字段一律 `int64`/`string`，**未触碰**
  `cool_kind` 的 int 枚举语义（乱改类型会让整个 state 文件 `Unmarshal` 失败而 `load()` 静默返回 → 数据全丢）。

## internal/pool/state.go
- `statusOf` 填入 `ClaimedTodayCredit: e.claimedCreditToday(now)`。
- 新增 `AddClaimedCredit(uid, n)`：跨天先清零再累加；`n<=0` 为空操作；写 `dirty`。
- 新增 `ClaimedTodayTotal()`：RLock 遍历，按当日对各账号求和（overview 顶层 `claimed_today`）。
- 原因：需求 2 的累加与聚合入口。

## internal/pool/persist.go
- `applyAccountsLocked`（load/快照恢复）：恢复 `claimedCredit` / `claimedDay`。
- `stateOverviewLocked`（落盘）：**仅当日**才写 `claimed_today_credit` / `claimed_day`
  （隔夜残留不写，避免陈旧计数跨天复活）。
- 原因：需求 2 要跨重启保留今日已领，同时跨天必须归零。

## internal/pool/pinned_claimed_test.go（新增）
- 10 个单元测试：钉号优先/被 disabled/冷却/暂停时回落、清空钉号、`tried` 优先、
  今日已领累计与求和、跨天归零、持久化往返、`PinnedUID` 裁剪空白。
- 原因：钉号「真的改了选号逻辑」需要可验证证据；这是本 fork 新增的唯一测试文件。

## internal/panel/panel.go
- `overview` 顶层新增 `pinned_uid`（`Pool.PinnedUID()`）与 `claimed_today`（`Pool.ClaimedTodayTotal()`）。
- `accountCheckin` 改用 `Upstream.DailyCheckinReward(a)`，成功时 `Pool.AddClaimedCredit(uid, credit)`。
- 原因：契约字段下发 + 需求 2 数据来源。

## internal/panel/tasks.go
- 新增统一入口 `noteClaimedCredit(uid, credit)`（`credit<=0` 空操作，内部调 `Pool.AddClaimedCredit`）。
- 手动领奖 `accountTaskClaim` 成功后调用它。
- 原因：需求 2 —— 任务领奖到账积分必须是**真实**数额才累加，设单一入口便于日后统一口径。

## internal/panel/autotask.go
- 8 处任务领奖成功点（自动/顺序/小程序任务、单任务动作、一键全部）在拿到 `credit` 后调用
  `p.noteClaimedCredit(...)`。
- 原因：同 tasks.go，覆盖所有自动领奖路径。

## internal/panel/taskcenter.go
- 成长任务队列自动领奖成功后 `p.noteClaimedCredit(a.UID, credit)`。
- 原因：同 tasks.go。

## internal/panel/import.go
- 批量导入账号时的签到改用 `DailyCheckinReward`，成功即累加今日已领。
- 原因：同 panel.go，保持签到到账的口径一致。

## internal/panel/login.go
- 新增账号登录时的首次签到改用 `DailyCheckinReward`，成功即累加今日已领。
- 原因：同上。

## internal/scheduler/scheduler.go
- `RunCheckinNow` 的签到改用 `DailyCheckinReward` 并累加 `AddClaimedCredit`；幂等（「今天已签到」）
  分支行为不变（仍只 `NoteCheckinDone`，不累加）。
- 原因：需求 2 的主签到排程路径。

## internal/scheduler/streak.go
- 连登管家：`ClaimGift`（新手礼包）、`ClaimCompensation`（补偿）、`GrowthRedeemTier`（连登档位兑换
  `tier.Credit`）三处成功后累加今日已领。
- 原因：这些接口返回的是**真实到账积分**，属「今日已领」的一部分。

## internal/upstream/client.go
- 新增 `DailyCheckinReward(a) (int64, error)`：捕获原本被丢弃的签到响应 `data`，调用
  `parseCheckinCredit` 解析到账积分。`DailyCheckin` 改为其薄包装（旧调用点/测试零改动）。
- 新增 `parseCheckinCredit(data json.RawMessage) int64`：按候选字段名
  （`credit` / `reward_credit` / `checkin_credit` / `credits` / `reward` / `sign_credit`，兼容 int/float）
  尽力解析；**解析不到任何字段即返回 0**。
- 原因（诚实说明，重要）：上游 `/v2/billing/meter/daily-checkin` 的响应 `data` 形状在本仓库
  **没有任何文档或样例**，原 `DailyCheckin` 直接丢弃响应体。因此这里只做尽力解析，
  **拿不到就记 0，绝不编造数字**。日后若拿到一次真实响应样例，把正确字段名补进
  `parseCheckinCredit` 的候选键即可（一行改动），无需改动其它结构。
  相对地，任务领奖 / 连登礼包 / 兑换的 `credit` 是仓库里明确存在、代码已在解析的真实数额，
  这些路径写入的是**真实已领积分**。

---

## scripts/probe_max_tokens.py（脱敏）
- docstring 用法示例：`--base http://211.154.25.123:7863/v1` → `--base http://127.0.0.1:7863/v1`。
- 原因（**主动脱敏**）：该示例里的公网 IP（端口 7863 = 本项目 panel 默认口）经与上游 v1.13.0 同名文件
  逐行比对，**上游原样存在**（jsdelivr 拉取
  `linguo2625469/workbuddy2api-panel@v1.13.0/scripts/probe_max_tokens.py` 第 18 行同为该 IP）。
  公开仓库不应出现真实可访问的主机地址（既可能被扫描滥用，也可能误伤他人），故本 fork 主动改为回环
  地址；仅示例语义，不影响脚本行为。

## .gitignore（构建正确性修复）
- 在 `*.md` / `!README.md` 之后新增例外 `!internal/prompt/defaultprompt.md`。
- 原因（**构建正确性**）：上游 `*.md` 规则会把 `internal/prompt/defaultprompt.md` 一并忽略，而该文件被
  `internal/prompt/prompt.go` 的 `//go:embed defaultprompt.md` **编译期内嵌**。若不入库，干净 clone 后
  `go build` 会直接报 `pattern defaultprompt.md: no matching files found`。加例外后干净 clone 可编译。

---

## go.mod（构建工具自动规范化，非手工改动）

`go build -mod=mod` 会在构建时按源码实际导入关系规范化 `require` 块：上游把被**直接导入**的
`github.com/redis/go-redis/v9 v9.18.0` 标成了 `// indirect`，工具链把它提升为独立的直接
`require`。属工具链规范化结果，依赖版本与集合**未变**。保留此规范化版本以保证「开箱即编译」。

---

## 未改动但需注意的上游文件

- `LICENSE` / `README.md` / `.dockerignore`：**沿用上游 v1.13.0 原样**，
  本 fork 尚未替换为自己的许可与说明（待仓库命名确定后处理）。
- `Dockerfile` / `docker-compose.yml` / `login.sh` / `signin.sh` / `credit.sh` / `scripts/*`：
  上游原样保留，供 Docker/CLI 部署参考（唯一例外：`scripts/probe_max_tokens.py` 已主动脱敏，见上）。

## 验证

本树可独立编译与测试（复用同一 Go 1.22.5 工具链 + 模块缓存）：
- `go build -trimpath -buildvcs=false -ldflags="-s -w" -o build/wb2api.exe ./cmd/server`
- `go test ./...`

> **`-buildvcs=false` 是可复现构建的必需项**：本仓库首版产物是在 `oss\` **尚无 `.git`** 时构建的
> （二进制不带 VCS 戳），而 Go 1.18+ 默认 `-buildvcs=true` 会把 `vcs.revision` / `vcs.time`
> 编进二进制 ⇒ 同一份源码在「有 `.git` 的 clone 里」与「无 `.git` 的源树里」会得到
> **字节数相同、md5 不同**的产物。要复现首版产物（md5 `5bafa496223ec008dbe387aacca5119e`，
> 8,693,760B），必须显式加 `-buildvcs=false`。
> （实测证据：干净 clone 内用不带该 flag 的命令得到 md5 `5f5351291489db5627d01e177edc4338`，
> 加上后即得 `5bafa496…`；两者大小均为 8,693,760B。）

---

## 已知限制与 Go 改造候选（壳侧 /api 适配层）

棱镜桌面壳在 Go 整合模式下由壳进程内的本地网关（`host/prism_localgw.py`）提供同源 `/api/*`。
原 flask 版（`棱镜PRISM.exe`）的「本地保险柜 / WorkBuddy 原生集成」能力**已逐条移植成真功能**
（账号保存、切换、交接包、批量导入导出、WorkDaddy 加密导入、解密环境自检、指定 WorkBuddy 位置、
一键部署模型目录），不再以「未内置」占位。仅以下一条经裁定**本轮不做**，在此显式记录为**已知限制**：

### 单号独立代理出口（per-account proxy）—— 已知限制，延后

- 相关 UI 端点：`POST /api/account/proxy`、`POST /api/account/proxy_test`。
- 现状：基座（上游 v1.13.0）**无 per-account 代理概念**——所有上游流量统一由基座进程发出，
  账号模型里没有「该号走哪个出口」的字段。壳侧无法在**不改 Go 源码**的前提下真正实现，
  故不伪造成功：这两个端点返回 `200 + ok:false + error:"not_in_build"` + 中文原因
  （**不是 501 死按钮**，UI 会如实展示「本版本不支持」）。
- 影响：UI 中该号的「独立代理」按钮不可用；多号「同 IP 关联风控」的缓解手段缺失。
- 临时替代：在基座全局上游（或系统层）统一设置代理；或运维层为不同实例分配不同出口。
- **Go 改造候选（待上游/本 fork 评估）**：在 `internal/pool` 的账号模型新增 `proxy` 字段，
  并在 `internal/upstream` 的每次上游请求里使用该号对应的 `http.Transport`（按 uid 缓存 client）。
  属 Go 侧改动，需单独立项。
