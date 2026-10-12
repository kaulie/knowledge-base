# knowledge-base

独立的统一知识库服务。管**源**（kind + provider），不反向依赖 Project 注册表。

Autonomy 用手里的 `project_id` 来取事实：内部先展开 `project_id → [knowledge_id]`，再检索这些源。Agent 入参不用改成 `knowledge_id`。

权限管控（`FORBIDDEN` / `access.mode`）接口已留，V1 不强制：按 project 绑定隔离即检索边界。

## 模型

| 对象 | 含义 |
|------|------|
| **knowledge** | 一条源。`knowledge_id` + kind/provider + 位置。登记一次，可挂到多个 project。 |
| **document / section** | 源里的文件与按 Markdown 标题切出的段。带 `revision` / `checksum`。 |
| **binding** | `project_id → [knowledge_id]`。Autonomy 只认 project_id。 |

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
GET    /api/knowledge/{id}
DELETE /api/knowledge/{id}
POST   /api/knowledge/{id}/sync
GET    /api/knowledge/{id}/documents
PUT    /api/knowledge/{id}/bindings/{projectID}
DELETE /api/knowledge/{id}/bindings/{projectID}
GET    /api/knowledge/{id}/bindings
GET    /api/backends
GET    /health
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
  "bind_project_id": "project-749a0238"
}
```

`sync` 回 `{status: indexed|unchanged, revision, checksum, documents, sections}`。源未变（checksum + revision）不重建。

## 读路径（给 Autonomy）

```
GET /api/projects/{projectID}/knowledge          # 等价 context.list
GET /api/projects/{projectID}/search?q=&limit=   # 等价 context.search
GET /api/sections/{sectionID}                    # 等价 context.get
```

没有绑定 → `knowledge: []` / `results: []`，不是错误。Autonomy 按现有约定走 `need_input`。

错误体一律 `{"code","message"}`：`INVALID_QUERY` `NOT_FOUND` `SOURCE_UNAVAILABLE` `PARSE_FAILED` `INDEX_FAILED`（`FORBIDDEN` 留给 ACL）。

## Autonomy 怎么接

`KNOWLEDGE_BASE_URL`（例如 `http://127.0.0.1:4360`）。`context.list` / `search` / `get` 仍从 `CONTEXT_ENTITY` 抄 `project_id`，HTTP 打到本服务。Knowledge **不**调用 `GET /api/projects`。
