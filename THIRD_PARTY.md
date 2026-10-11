# 第三方来源与许可证清单（THIRD_PARTY）

本文件列举本仓库**直接包含或依赖**的第三方成果及其许可证，供合规审查与再分发时参考。
本仓库自身以 MIT 许可发布，见 [`LICENSE`](./LICENSE)。

> 说明：下表版本以 `server/go.mod` 与 `server/go.sum` 为准。Go 语言标准库随工具链分发，
> 采用 Go 项目自身的 BSD-3-Clause 许可，此处不逐条展开。

## 一、上游项目（fork 来源）

| 项目 | 作者 / 仓库 | 许可证 | 关系 |
|---|---|---|---|
| workbuddy2api | [Sliverkiss/workbuddy2api](https://github.com/Sliverkiss/workbuddy2api) | MIT | **原始项目；仓库已删除**。本仓库同步至其删库前最后一次更新 |
| workbuddy2api-panel v1.13.0 | [linguo2625469/workbuddy2api-panel](https://github.com/linguo2625469/workbuddy2api-panel) | MIT | **直接上游 fork**，本仓库以其为基座 |

- 本仓库 `server/` 目录的**绝大多数源码**来自上述上游，其原始版权与许可声明原样保留。
- 本 fork 相对上游 v1.13.0 的改动见 [`CHANGES-vs-upstream.md`](./CHANGES-vs-upstream.md)。

## 二、Go 运行时依赖（`server/go.mod`）

| 模块 | 版本 | 许可证 | 用途 |
|---|---|---|---|
| `github.com/redis/go-redis/v9` | v9.18.0 | BSD-2-Clause | 可选 Upstash / Redis 状态镜像 |
| `github.com/cespare/xxhash/v2` | v2.3.0 | MIT | go-redis 间接依赖（哈希） |
| `github.com/dgryski/go-rendezvous` | `v0.0.0-20200823014737-9f7001d12a5f` | MIT | go-redis 间接依赖（一致性哈希） |
| `go.uber.org/atomic` | v1.11.0 | MIT | go-redis 间接依赖（原子类型） |
| `golang.org/x/sys` | v0.30.0 | BSD-3-Clause | go-redis 间接依赖（系统调用封装） |

> 备注：`server/go.sum` 中另含 `go-redis v9.22.0` 的校验行，属模块图解析产生的 checksum 记录；
> **实际选中并参与构建的版本为 `go.mod` 所钉的 `v9.18.0`**，`go.mod` 未提升到 v9.22.0。

## 三、代码随附的脚本与资源

| 内容 | 位置 | 来源 | 许可证 |
|---|---|---|---|
| Shell 脚本 `login.sh` / `signin.sh` / `credit.sh` | `server/` | 上游 workbuddy2api-panel | 随上游，MIT |
| Python 辅助脚本 `scripts/*.py` | `server/scripts/` | 上游 workbuddy2api-panel | 随上游，MIT |
| Docker 构建文件 `Dockerfile` / `docker-compose.yml` | `server/` | 上游 workbuddy2api-panel | 随上游，MIT |
| README 图标图片（外链引用） | `server/README.md` | `DGZSbot/ai-icon`（raw.githubusercontent.com） | 外链引用，未随仓库分发；版权归原作者，使用时请自行确认 |

## 四、外部服务（非本仓库分发物）

| 服务 | 说明 |
|---|---|
| 腾讯 CodeBuddy（`copilot.tencent.com`） | 本项目通过 **OAuth 设备授权** 访问的**上游服务**。本项目**不包含、不分发**其任何代码或二进制，仅为客户端集成；使用须遵守其服务条款 |
| Upstash Redis（可选） | 由用户自行配置的第三方托管 Redis，用于状态镜像；本项目仅作为客户端调用 |

## 五、再分发注意

1. 再次分发本仓库（或其衍生版本）时，**必须保留**：
   - 本仓库 [`LICENSE`](./LICENSE) 中 `Sliverkiss`、`linguo2625469` 与本 fork 的全部 copyright 行；
   - 本文件所列第三方模块的许可证声明（尤其 BSD 类许可要求保留版权与免责声明）。
2. 请勿移除 `server/` 内上游源码文件头部的版权注释。
3. **不要**把用户的真实凭证（`auths/`、`vault.json`、`data/state.json`）一并分发 —— 参见顶层 [`.gitignore`](./.gitignore) 已把这些路径排除。

---

*本清单为人工整理，若发现遗漏或与上游实际声明不符，欢迎提交 Issue / PR 更正。*
