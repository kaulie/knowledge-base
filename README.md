# knowledge-base

独立的统一知识库服务。管**源**（kind + provider），不反向依赖 Project 注册表。

绑定对象是 **CONTEXT entity**：`{type, id}`。`project` 只是其中一种；后面可以挂 `organization` / `service` / `environment` / `task`，或任何符合 `[a-z][a-z0-9_]*` 的新 type，服务端不用改代码。

Autonomy 用手里的 entity 取事实：内部先展开 `{type,id} → [knowledge_id]`，再检索这些源。Agent 入参不用改成 `knowledge_id`。

权限管控（`FORBIDDEN` / `access.mode`）接口已留，V1 不强制：按 entity 绑定隔离即检索边界。

## 模型

| 对象 | 含义 |
|------|------|
| **knowledge** | 一条源。`knowledge_id` + kind/provider + 位置。登记一次，可挂到多个 entity。 |
| **document / section** | 源里的文件与按 Markdown 标题切出的段。带 `revision` / `checksum`。 |
| **context entity** | `{type, id}`。`project` 是一种；同一 `id` 在不同 `type` 下互不看见。 |
| **binding** | `{type,id} → [knowledge_id]`。读路径只认 entity，不认 Project 注册表。 |

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
# KNOWLEDGE_BASE_ADDR=:4360
```

V1 索引在进程内存里，重启即空。持久化（postgres）与远程连接器后加。

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
```

`PUT /api/knowledge/{id}/bindings/{projectID}` 是 `type=project` 的别名。

登记示例（通用 bind）：

```json
{
  "name": "autonomy docs",
  "kind": "filesystem",
  "provider": "local",
  "shape": "directory",
  "source": {"location": "/srv/docs"},
  "suffixes": ["md"],
  "bind": {"type": "project", "id": "project-749a0238"}
}
```

`bind_project_id` 仍可用，等价于 `bind: {type: project, id: …}`。

`sync` 回 `{status: indexed|unchanged, revision, checksum, documents, sections}`。源未变（checksum + revision）不重建。

## 读路径（给 Autonomy）

```
GET /api/contexts/{entityType}/{entityID}/knowledge   # 等价 context.list
GET /api/contexts/{entityType}/{entityID}/search?q=   # 等价 context.search
GET /api/sections/{sectionID}                         # 等价 context.get
```

`/api/projects/{projectID}/…` 是 `entityType=project` 的别名，方便现有 Autonomy 先只传 project。

没有绑定 → `knowledge: []` / `results: []`，不是错误。Autonomy 按现有约定走 `need_input`。

错误体一律 `{"code","message"}`：`INVALID_QUERY` `NOT_FOUND` `SOURCE_UNAVAILABLE` `PARSE_FAILED` `INDEX_FAILED`（`FORBIDDEN` 留给 ACL）。

## Autonomy 怎么接

`KNOWLEDGE_BASE_URL`（例如 `http://127.0.0.1:4360`）。`context.list` / `search` / `get` 从 `CONTEXT_ENTITY` 抄 `{type, id}`（今天 type 多半是 `project`），HTTP 打到 `/api/contexts/{type}/{id}/…`。Knowledge **不**调用 `GET /api/projects`。
