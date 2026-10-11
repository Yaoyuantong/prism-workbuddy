# 棱镜 PRISM · WorkBuddy 多账号网关（开源分支）

> 把腾讯 CodeBuddy 账号变成 **OpenAI 兼容 API** 的多账号自托管网关，附 Web 管理面板，
> 并配套一个 Windows 桌面壳（棱镜 PRISM）把基座、本地网关与图形界面打包成一个开箱即用的工具。

- **本仓库是 fork**：基于 [`linguo2625469/workbuddy2api-panel`](https://github.com/linguo2625469/workbuddy2api-panel) v1.13.0（MIT）。
- 该项目的**原始上游** [`Sliverkiss/workbuddy2api`](https://github.com/Sliverkiss/workbuddy2api) **已删库**，本分支独立维护演进。
- 我们相对上游 v1.13.0 的**逐文件改动清单**见 [`CHANGES-vs-upstream.md`](./CHANGES-vs-upstream.md)。
- **纯开源、永久免费**。仅限**本人授权账号 + 本机 / 私有环境**使用，**禁止**批量注册小号、转售配额、加壳捆绑收费（详见文末[使用声明与合规边界](#使用声明与合规边界)）。

---

## 项目简介

WorkBuddy2API 是一个**自托管的 OpenAI 兼容反向代理网关**：官方并不提供 OpenAI 形态的开放 API，
本项目通过 **OAuth 设备授权**取得账号凭证，在网关侧完成 token 自动刷新、账号池调度与流量治理，
对外只暴露标准的 `/v1/chat/completions`，让现有 OpenAI SDK / 前端 / 工具**零改造接入**。

面向**个人多账号**场景：多账号共享、单号故障自动换号、冷却 / 熔断防雪崩、会话粘性保证多轮上下文不跳号；
并内置定时签到、保活、成长任务等运维能力，配一个可视化 Web 面板。

## 核心功能

| 能力 | 说明 |
|---|---|
| 多账号池 | 快过期积分加权 + 成本分层 + 加权随机选号，Top-5 候选 + 防惊群（`minPickGap`） |
| 熔断与冷却 | 429 软冷却指数退避、404 短冷却、402 硬冷却至次日 04:00、连败熔断、在途限流 |
| 会话粘性 | 同一 `conversation_id` 尽量绑定同一账号，TTL 滚动续期，失败自动解绑 |
| 定时任务 | 签到（09/21 点，末尾跑连登管家）、活跃上报、猫猫旅行、token 保活，分类独立开关 |
| 流式 / 非流式 | 出站强制 `stream:true`；SSE 帧白名单重建；非流式本地聚合为单响应 |
| 推理模型兼容 | DeepSeek 思维链注入、`reasoning_content` 多轮回填、effort 档位自动降级 |
| 提示词体系 | 网关自有提示词替换客户端 system，遇拦截自动降级重试 |
| 指纹脱敏 | 出站请求体黑名单指纹字段清洗（可关闭） |
| 可观测 | 每请求一行表格日志（TTFB / token 速率 / uid）、`/healthz` 带身份标识可接探活 |
| 状态持久化 | 池状态本地原子落盘 + 可选 Upstash Redis 镜像，重启择新恢复 |
| **钉号（本 fork 新增）** | 指定网关固定走某个账号，改配置**立即热生效**，账号不可用时**自动回落**轮转 |
| **今日已领积分（本 fork 新增）** | 账号级今日到账积分累计，面板总览透出 `claimed_today` |

> 钉号与「今日已领积分」是上游没有、由本 fork 增加的字段，接口契约与实现细节见
> [`CHANGES-vs-upstream.md`](./CHANGES-vs-upstream.md)。

## 混合架构说明

本项目在运行时由**三层**协作，仓库中的 `server/` 只是其中最核心的一层：

```
┌─────────────────────────────┐
│  桌面壳（棱镜 PRISM / Win）  │  图形界面：总览 / 账号保险柜 / 网关 / 任务 / 日志
│  - 渲染 UI、本地账号保险柜    │  同源反代 /api/* → 本地网关
│  - 拉起并守护基座进程         │
└───────────────┬─────────────┘
                │ HTTP（仅本机）
┌───────────────▼─────────────┐
│  本地网关 prism_localgw      │  同源聚合层：把 UI 请求映射到基座 REST 接口
│  （纯透传 / 字段适配）        │  统一鉴权、路径、字段形状
└───────────────┬─────────────┘
                │ REST
┌───────────────▼─────────────┐
│  Go 基座 workbuddy2api-panel │  ★ 本仓库 server/：账号池 + 反向代理 + 面板 REST
│  - /v1/chat/completions      │  对外 OpenAI 兼容
│  - /panel/api/*  /healthz    │  面板数据接口
└─────────────────────────────┘
```

- **Go 基座**（`server/`）：账号池、反向代理、面板 REST，单二进制、无运行时依赖，**本仓库的核心源码**。
- **本地网关**：桌面壳与基座之间的同源透传 / 字段适配层，负责把 UI 用到的接口形状对齐到基座契约。
- **桌面壳**：Windows 图形界面与账号保险柜（本地加密存储），负责拉起与守护基座进程。

> **当前仓库内容**：本轮已收录 **Go 基座（`server/`）**。桌面壳与 UI 源将随后续提交纳入，
> 建议的顶层目录布局见下节。

## 目录结构

**当前（本仓库现已收录）：**

```
.
├── LICENSE                     # MIT，保留上游原文并追加本 fork copyright
├── README.md                   # 本文件
├── THIRD_PARTY.md              # 第三方来源与许可证清单
├── CHANGES-vs-upstream.md      # 相对上游 v1.13.0 的逐文件改动清单
├── .gitignore
└── server/                     # Go 基座源码（fork 自 workbuddy2api-panel v1.13.0）
    ├── cmd/                    # server / login / credit / signin / trial 等入口
    ├── internal/               # pool / panel / scheduler / upstream / auth ...
    ├── scripts/                # 上游辅助脚本（Python / Shell）
    ├── go.mod  go.sum
    ├── config.example.json     # 配置样例（不含任何密钥）
    ├── Dockerfile  docker-compose.yml
    └── README.md               # ★ 上游原始 README，原样保留，作为上游能力参考
```

**建议（后续提交逐步补齐，本轮不搬文件）：**

```
prism-workbuddy/
├── LICENSE  README.md  THIRD_PARTY.md  CHANGES-vs-upstream.md  .gitignore
├── server/                     # Go 基座（当前已在此）
├── shell/                      # 桌面壳源码（棱镜 PRISM：Electron/Win 壳 + 账号保险柜 + 本地网关）
├── ui/                         # 界面源（Web UI 前端工程）
├── packaging/                  # 打包脚本：壳 + 基座 + 本地网关 → 单安装包
└── docs/                       # 设计/评审/协议文档
```

> 顶层放**文档与许可**，`server/` 放 Go 基座，`shell/` 放桌面壳，`ui/` 放前端工程，
> `packaging/` 放打包脚本 —— 三类产物各自独立、可单独构建，避免把源码、产物与凭证混在一起。

## 构建与安装

### 1. 构建 Go 基座（`server/`）

要求 **Go 1.22.5**（`go.mod` 指定版本），无需 CGO：

```bash
cd server

# Windows（amd64）
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o build/wb2api.exe ./cmd/server

# Linux（amd64）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o build/wb2api ./cmd/server
```

运行与自检：

```bash
cp config.example.json config.json      # 按需修改，切勿提交 config.json
./build/wb2api.exe -config config.json  # 默认读取当前目录 config.json

curl --noproxy '*' http://127.0.0.1:7863/healthz          # 健康检查
curl --noproxy '*' http://127.0.0.1:7863/panel/           # 打开面板
curl --noproxy '*' http://127.0.0.1:7863/panel/api/overview
```

### 2. Docker 部署（可选）

`server/` 保留了上游的 `Dockerfile` 与 `docker-compose.yml`：

```bash
cd server
docker compose up -d --build
```

### 3. 桌面壳安装包（后续）

棱镜 PRISM 桌面壳 + 本地网关 + Go 基座的**单文件安装包**由 `packaging/` 脚本产出，
后续提交随壳与 UI 源码一并纳入本仓库。

## 来源与致谢

本项目站在开源社区的肩膀上，特此致谢：

- **原始项目**：[`Sliverkiss/workbuddy2api`](https://github.com/Sliverkiss/workbuddy2api) —— 账号池调度、错误分类、提示词体系等核心设计的源头。**该仓库已删除**，本项目同步至其删库前的最后一次更新，并在此基础上重构可视化运维层。
- **直接上游 fork**：[`linguo2625469/workbuddy2api-panel`](https://github.com/linguo2625469/workbuddy2api-panel) v1.13.0 —— 本仓库以其为基座（MIT）。
- 上游设计的精巧之处（加权选号、成本分层、冷却熔断、会话粘性、提示词体系）**原样保留**；本 fork 的改动详见 [`CHANGES-vs-upstream.md`](./CHANGES-vs-upstream.md)。
- 第三方依赖库及其许可证清单见 [`THIRD_PARTY.md`](./THIRD_PARTY.md)。

## 使用声明与合规边界

> 以下声明与上游 [Sliverkiss/workbuddy2api](https://github.com/Sliverkiss/workbuddy2api) 及 linguo2625469 分支的立场一致，本 fork 完全继承并重申。

本项目的开发初衷只有一个：**方便个人管理自己的 CodeBuddy 账号** —— 自动签到、保活、给自己的本地工具提供一个 OpenAI 兼容入口。它是免费、开源、按「原样」提供的**个人自用工具**。

**使用边界（务必遵守）：**

1. **仅限本人授权账号、本机 / 私有环境。** 本项目是**非官方**网关，以 CodeBuddy 账号作为上游，请仅用于你本人拥有并获授权的账号，勿在公开环境暴露。
2. **禁止批量注册小号 / 收购账号，对外提供付费 API、共享池、代充等业务。** 此行为违反目标平台服务条款，与作者无关，后果自负。
3. **禁止二次加壳、捆绑卡密（授权码）售卖，或以「公益服 / 低价中转」等名义变相收费分发。** 本项目**永远免费开源**，任何加壳、加密、捆绑收费的「版本」都是他人篡改的产物，与本项目无关。
4. **请勿购买任何「收费版」「卡密版」「公益中转版」。** 此类分发无法审计，存在被植入后门、回传并窃取你 CodeBuddy 凭证的风险（`auths/` 中保存的是**明文** accessToken / refreshToken）。**你付钱买到的不是本项目，而是把自己账号交给陌生人的机会。**
5. **协议允许自由使用与修改源码**（这是开源的本意），但 MIT 赋予的是**代码层面**的自由，**不赋予**以本项目名义宣传、售卖、捆绑分发，或要求作者提供支持与背书的权利。作者不为任何第三方分发版本提供支持、更新承诺或安全保证。
6. **作者保留止损的权利。** 若滥用行为持续，作者可能随时停止维护、关闭或删除仓库，且不另行通知。

如果你的用途是管理自己的账号，欢迎正常使用、反馈问题与提交 PR。

## 许可

本项目基于 **MIT License** 开源，完整文本见 [`LICENSE`](./LICENSE)。该文件**保留了上游 `Sliverkiss` 与 `linguo2625469` 的原始 copyright 行**，并在其后追加了本 fork 的 copyright。

```
Copyright (c) 2026 Sliverkiss (original project: Sliverkiss/workbuddy2api)
Copyright (c) 2026 linguo2625469 (this fork: linguo2625469/workbuddy2api-panel)
Copyright (c) 2026 prism-workbuddy contributors (this fork)
```

> MIT 协议允许自由使用、修改与再分发源码；再次分发时**必须保留上述版权声明与本许可声明**。
