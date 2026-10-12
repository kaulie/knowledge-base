# knowledge-base

独立的统一知识库服务。管**源**（kind + provider）。调用方是谁、登记表在哪，这里不管。

绑定对象只有 **CONTEXT entity**：`{type, id}`。`project` / `organization` / `service` / `environment` / `task` 都是 type 的取值，地位相同。任何符合 `[a-z][a-z0-9_]*` 的新 type 都可以，服务端不用改代码。没有 `project_id` 字段，也没有 `/api/projects` 路径。

Autonomy 用手里的 entity 取事实：内部先展开 `{type,id} → [knowledge_id]`，再检索这些源。Agent 入参不用改成 `knowledge_id`。

权限管控（`FORBIDDEN` / `access.mode`）接口已留，V1 不强制：按 entity 绑定隔离即检索边界。

## 模型

| 对象 | 含义 |
|------|------|
| **knowledge** | 一条源。`knowledge_id` + kind/provider + 位置。登记一次，可挂到多个 entity。 |
| **document / section** | 源里的文件与按 Markdown 标题切出的段。带 `revision` / `checksum`。 |
| **context entity** | `{type, id}`。同一 `id` 在不同 `type` 下互不看见。 |
| **binding** | `{type,id} → [knowledge_id]`。 |

### kind + provider

| kind | 已实现 | 未实现（`INVALID_QUERY`） |
|------|--------|---------------------------|
| `filesystem` | `local` | `sftp` / `ssh` |
| `source_code_repository` | `git`（绑定时 `git clone`，按仓库树解析） | `github` / `gitlab` / `gitea` |
| `cloud` | （无） | `s3` / `gcs` / `azure` / `oss` |

## 运行

```bash
go test ./src
go run ./cmd/knowledge-based
# KNOWLEDGE_BASE_ADDR=127.0.0.1:4360
```

V1 索引在进程内存里，重启即空。持久化（postgres）与远程连接器后加。

维护页：`GET /` 或 `GET /dashboard`。按 CONTEXT entity `{type, id}` 登记、绑定、同步。选 `project` 时项目列表来自 web-cursor `GET /api/projects`（本服务只代理，`PROJECTS_API_URL`，默认 `http://127.0.0.1:4211`），不用手填。

检索页：`GET /search`。列出已绑定的 context，以及每个下面已索引的文件与标题。

## 服务中心与部署

契约真源是 handler 上的 swag 注解（`cmd/knowledge-based/main.go` + `src/http.go`）。`src/contract_test.go` 保证注解与路由表一一对应。

```
bash scripts/register-contract.sh
# SERVICE_NAME=knowledge-base  DEPARTMENT_ID=D0005  INSTANCES=127.0.0.1:4360
# → 服务中心 :4240
```

部署平台（`:4220`）绑本服务后走统一发版：

```
POST /api/deploy-notify  {"serviceId":"knowledge-base","ref":"main"}
```

平台调仓库根 `build.sh` → `outputs/`（含 `bin/knowledge-based` 与 `scripts/restart.sh`），rsync 到 runtime，再执行 `restartCmd`。监听端口 **4360**。

## 写路径（登记 / 维护 / 版本）

```
POST   /api/knowledge
GET    /api/knowledge
GET    /api/knowledge?entity_type=&entity_id=
GET    /api/knowledge/{id}
DELETE /api/knowledge/{id}
POST   /api/knowledge/{id}/sync
GET    /api/knowledge/{id}/documents
PUT    /api/knowledge/{id}/bindings/{entityType}/{entityID}
DELETE /api/knowledge/{id}/bindings/{entityType}/{entityID}
GET    /api/knowledge/{id}/bindings
GET    /api/backends
GET    /api/catalog/entities/{entityType}
GET    /api/contexts
GET    /api/contexts/{entityType}/{entityID}/browse
GET    /health
GET    /
GET    /dashboard
GET    /search
```

登记示例：

```json
{
  "name": "autonomy docs",
  "kind": "filesystem",
  "provider": "local",
  "shape": "directory",
  "source": {"location": "/srv/docs"},
  "suffixes": ["md"],
  "bind": {"type": "project", "id": "749a0238"}
}
```

源码仓库必须填 **git 仓库地址**。仓库内 path 从**仓库根目录**写起，不是本机路径。前导 `/` 可有可无：`docs/runtime.md` 与 `/docs/runtime.md` 相同，登记后统一存成无前导斜杠。空 path = 整棵树；`shape: file` 必须写到具体文件。

```json
{
  "name": "autonomy",
  "kind": "source_code_repository",
  "provider": "git",
  "shape": "directory",
  "source": {
    "location": "git@github.com:kaulie/autonomy.git",
    "ref": "main",
    "path": "docs"
  },
  "suffixes": ["md"],
  "bind": {"type": "project", "id": "749a0238"}
}
```

`source.location` 或 `source.repository` 是仓库地址（必填），`source.ref` 可选。绑定（或带 `bind` 的登记）会 clone 到 `KNOWLEDGE_BASE_DATA/repos/{knowledge_id}`（默认 runtime `backend/data`），再按 `git ls-tree` 解析跟踪文件。不是本机已有 checkout，也不是 `git-local`。

`sync` 回 `{status: indexed|unchanged, revision, checksum, documents, sections, logs}`。`logs` 是逐步过程（clone / fetch / 列举 / 解析每个文件 / 结束），维护页同步后以日志面板展示，进程日志里也有同样一行。失败时错误体同样带 `logs`。源未变（checksum + revision）不重建。源码仓库的 `sync` 会 fetch 后再建索引。源码仓库的 bind 会先 sync，响应里带 `sync.logs`。

## 读路径（给 Autonomy）

```
GET /api/contexts                                     # 已绑定 entity 清单
GET /api/contexts/{entityType}/{entityID}/browse      # 该 entity 下已索引文件与标题
GET /api/contexts/{entityType}/{entityID}/knowledge   # 等价 context.list
GET /api/contexts/{entityType}/{entityID}/search?q=   # 等价 context.search
GET /api/sections/{sectionID}                         # 等价 context.get
```

维护页 `/` 管登记与绑定。检索页 `/search` 列出已绑定的 CONTEXT entity，以及每个下面有哪些文档和标题，并可按词检索。

没有绑定 → `knowledge: []` / `results: []`，不是错误。Autonomy 按现有约定走 `need_input`。

错误体一律 `{"code","message"}`：`INVALID_QUERY` `NOT_FOUND` `SOURCE_UNAVAILABLE` `PARSE_FAILED` `INDEX_FAILED`（`FORBIDDEN` 留给 ACL）。

## Autonomy 怎么接

`KNOWLEDGE_BASE_URL`（例如 `http://127.0.0.1:4360`）。`context.list` / `search` / `get` 从 `CONTEXT_ENTITY` 抄 `{type, id}`，HTTP 打到 `/api/contexts/{type}/{id}/…`。本服务不查询任何调用方注册表。
