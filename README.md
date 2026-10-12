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
| `source_code_repository` | `git-local`（本机 checkout） | `github` / `gitlab` / `gitea` |
| `cloud` | （无） | `s3` / `gcs` / `azure` / `oss` |

## 运行

```bash
go test ./src
go run ./cmd/knowledge-based
# KNOWLEDGE_BASE_ADDR=127.0.0.1:4360
```

V1 索引在进程内存里，重启即空。持久化（postgres）与远程连接器后加。

维护页：`GET /` 或 `GET /dashboard`。按 CONTEXT entity `{type, id}` 登记、绑定、同步、检索。

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
GET    /health
GET    /
GET    /dashboard
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

`sync` 回 `{status: indexed|unchanged, revision, checksum, documents, sections}`。源未变（checksum + revision）不重建。

## 读路径（给 Autonomy）

```
GET /api/contexts/{entityType}/{entityID}/knowledge   # 等价 context.list
GET /api/contexts/{entityType}/{entityID}/search?q=   # 等价 context.search
GET /api/sections/{sectionID}                         # 等价 context.get
```

没有绑定 → `knowledge: []` / `results: []`，不是错误。Autonomy 按现有约定走 `need_input`。

错误体一律 `{"code","message"}`：`INVALID_QUERY` `NOT_FOUND` `SOURCE_UNAVAILABLE` `PARSE_FAILED` `INDEX_FAILED`（`FORBIDDEN` 留给 ACL）。

## Autonomy 怎么接

`KNOWLEDGE_BASE_URL`（例如 `http://127.0.0.1:4360`）。`context.list` / `search` / `get` 从 `CONTEXT_ENTITY` 抄 `{type, id}`，HTTP 打到 `/api/contexts/{type}/{id}/…`。本服务不查询任何调用方注册表。
