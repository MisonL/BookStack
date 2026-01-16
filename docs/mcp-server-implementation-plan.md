# BookStack MCP Server 实施方案

> 基于现有 BookStack 结构（Beego 应用，文档内容在 `document_store.go`/`md_document_store`，搜索逻辑在 `document_search_result.go`、`elasticsearch.go`），为 Agent 提供类似 Context7 的信息获取能力。

---

## 方案概览

| 项目         | 说明                                                                             |
| ------------ | -------------------------------------------------------------------------------- |
| **目标**     | 提供 MCP Server，让 Agent 可检索书籍/文档、获取内容片段，并遵守书籍私有/权限规则 |
| **推荐架构** | 独立 MCP Sidecar（与 BookStack 同库/同配置），最小侵入、易于部署与回滚           |
| **备选架构** | 内置到 BookStack 进程里（同 HTTP 服务或独立端口），开发更省但耦合更深            |

---

## MCP 能力设计

### Resources（推荐）

| Resource URI                                  | 说明                                |
| --------------------------------------------- | ----------------------------------- |
| `bookstack://books`                           | 书籍列表（支持分页/分类/私有过滤）  |
| `bookstack://books/{book_id}`                 | 书籍元信息                          |
| `bookstack://books/{book_id}/docs`            | 文档目录树/扁平列表                 |
| `bookstack://docs/{doc_id}`                   | 文档全文（markdown/text/html 可选） |
| `bookstack://docs/{doc_id}#section={heading}` | 按标题段落切分后的内容              |
| `bookstack://recent`                          | 最近更新文档                        |

## 详细技术实现方案

### 1. 数据模型映射与复用

基于 `models` 包的分析，MCP Server 将通过以下方式访问数据（Sidecar 模式下建议直连数据库以获得高性能，或通过 Go 代码复用 `models`）：

#### 核心数据表

| BookStack Model | 数据库表            | 关键字段                                                                             | 用途                                           |
| --------------- | ------------------- | ------------------------------------------------------------------------------------ | ---------------------------------------------- |
| `Book`          | `md_books`          | `book_id`, `book_name`, `identify`, `description`, `privately_owned`, `doc_count`    | 书籍元数据，权限判断(`privately_owned`)        |
| `Document`      | `md_documents`      | `document_id`, `book_id`, `document_name`, `identify`, `release` (HTML), `parent_id` | 文档树结构，HTML 内容                          |
| `DocumentStore` | `md_document_store` | `document_id`, `markdown`, `content`                                                 | **文档源文件** (优先读取此表以获取 clean text) |
| `Member`        | `md_members`        | `member_id`, `account`, `role`                                                       | 用户身份信息                                   |
| `Relationship`  | `md_relationship`   | `book_id`, `member_id`, `role_id`                                                    | 私有书籍的访问权限关联                         |

> **关键策略**：读取文档内容时，Tool `get_doc` 应优先查询 `md_document_store.markdown`。如果为空，再降级使用 `md_documents.release` (HTML) 并转换为 Markdown/Text，以提供给 LLM 更优质的上下文。

### 2. 搜索逻辑适配

目前的搜索逻辑分布在 `controllers/SearchController.go` 和 `models/document_search_result.go`。MCP 将复用此逻辑：

- **混合搜索策略**：
  1. 检查配置 `ELASTICSEARCH_ON` (在 `conf/app.conf` 或环境变量)。
  2. **Enabled**: 调用 `models.NewElasticSearchClient().Search(...)`。
     - 索引结构参考 `models/elasticsearch.go` 中的 `ElasticSearchData`。
  3. **Disabled**: 降级为 SQL `LIKE` 查询。
     - 复用 SQL 逻辑：`SELECT ... FROM md_documents WHERE document_name LIKE %?% OR release LIKE %?%`。
     - 注意过滤 `privately_owned=0` 除非提供了合法的 `member_token`。

### 3. 工具 (Tools) 详细定义

#### `list_books`

- **SQL 逻辑**: `SELECT * FROM md_books WHERE inputs.category MATCH ...`
- **权限过滤**:
  - 默认仅返回 `privately_owned=0`。
  - 若传入 `member_id`，额外返回 `md_relationship` 中匹配的书籍。

#### `get_doc`

- **逻辑**:
  1. 根据 input `doc_id` 或 `identify` (结合 `book_id`) 查询 `md_documents` 获取基础信息。
  2. 查询 `md_document_store` 获取 `markdown` 字段。
  3. 如果 `format=html`，返回 `md_documents.release`。
  4. 权限校验：检查书籍是否公开，或用户是否有权。

#### `search_docs`

- **参数**: `query` (string), `book_id` (optional int)
- **返回**:
  ```json
  [
    {
      "id": 123,
      "title": "文档标题",
      "snippet": "搜索匹配的文本片段...",
      "score": 1.2,
      "book_id": 45
    }
  ]
  ```

### 4. 鉴权方案 (Refined)

为了支持 Sidecar 模式且保持灵活性，采用 **双层鉴权**：

1. **Service Level**: `MCP_SERVICE_TOKEN`

   - 用途：防止未经授权的客户端连接 MCP Server。
   - 实现：环境变量配置，客户端连接时需在 headers 或 init params 中携带。

2. **User Level** (Context-Aware):
   - 用途：Agent 代表特定用户操作（如搜索私有文档）。
   - 实现：
     - 复用 `md_members` 表。
     - Tool 调用时接受 `member_token` 或 `api_token` 参数。
     - MCP Server 验证 Token 有效性（参考 `models/member_token.go` 或 `models.Member.Valid`），解析出 `member_id` 进行后续 SQL 过滤。

---

## 推荐工程结构 (针对 Sidecar/Go)

建议在现有仓库中新建 `cmd/mcp-server`，直接引用现有 `models` 包，从而最大化重用代码。

```text
BookStack/
├── cmd/
│   └── mcp-server/      # [NEW] MCP Server 入口
│       ├── main.go      # 启动逻辑，加载 conf
│       └── server.go    # MCP 协议处理与路由
├── mcp/                 # [NEW] MCP 核心逻辑
│   ├── tools.go         # 具体的 Tool 实现 (Search, GetDoc)
│   ├── resources.go     # Resource 实现
│   └── auth.go          # 鉴权中间件
├── models/              # [EXISTING] 复用数据模型
└── conf/                # [EXISTING] 复用配置文件读取
```

这个结构允许 MCP Server 作为一个独立的二进制文件编译 (`go build -o mcp-server ./cmd/mcp-server`)，同时享受现有项目的 ORM 和配置管理。

### Tools（推荐）

| Tool               | 参数                                             | 说明                        |
| ------------------ | ------------------------------------------------ | --------------------------- |
| `search_docs`      | `query`, `book_id?`, `limit?`, `member_token?`   | 全文检索并返回片段 + doc_id |
| `get_doc`          | `doc_id`, `format=markdown\|text\|html`          | 按需拉取完整内容            |
| `list_books`       | `category?`, `include_private?`, `member_token?` | 书籍列表                    |
| `list_docs`        | `book_id`, `tree=true\|false`                    | 文档列表/目录               |
| `get_book_outline` | `book_id`                                        | 树状目录                    |

### Prompts（可选）

| Prompt                  | 说明                          |
| ----------------------- | ----------------------------- |
| `summarize_doc(doc_id)` | 生成摘要（配合 Agent 侧使用） |

---

## 检索与内容生成

| 类别         | 方案                                                                    |
| ------------ | ----------------------------------------------------------------------- |
| **默认检索** | 复用 `document_search_result.go` 的 SQL LIKE 搜索（易落地）             |
| **可选增强** | 接入现有 ES（`elasticsearch.go`），有则用 ES，否则降级 SQL              |
| **内容来源** | 优先 `DocumentStore.Content` 或 `DocumentStore.Markdown`，避免仅用 HTML |
| **切分策略** | 按 Markdown 标题切分（或固定字数切分），产出 `section_id` 便于引用      |

---

## 权限与安全

| 类别     | 方案                                                                          |
| -------- | ----------------------------------------------------------------------------- |
| **身份** | 新增 MCP Token（配置项或数据库表）；或复用已有成员系统（`member_id` + token） |
| **授权** | 遵循 `books.privately_owned` 和 `relationship` 角色校验                       |
| **隔离** | 只读访问；限制 IP 白名单/速率；支持只公开内容模式                             |

---

## 实施步骤（里程碑）

| 阶段        | 目标                                                                           |
| ----------- | ------------------------------------------------------------------------------ |
| **M0 基线** | 确定传输方式（stdio/HTTP SSE）、部署形态（sidecar/内置）、鉴权方案             |
| **M1 MVP**  | MCP Server 骨架 + `list_books`/`list_docs`/`get_doc`/`search_docs`（SQL 检索） |
| **M2 权限** | 接入角色校验、私有书籍访问控制、审计日志                                       |
| **M3 体验** | 加入 Markdown 分段/引用、缓存、ES 可选增强                                     |
| **M4 文档** | 配置说明、Agent 侧接入示例、运维与监控                                         |

---

## 配置建议

```yaml
# MCP Server 配置
mcp.enabled: true
mcp.bind: "0.0.0.0:9090"
mcp.auth_token: "your-secure-token"
mcp.allow_private: false

# Elasticsearch（可选）
ELASTICSEARCH_ON: true
ELASTICSEARCH_HOST: "http://localhost:9200"
```

---

## 风险与规避

| 风险         | 规避措施                                                |
| ------------ | ------------------------------------------------------- |
| **隐私泄露** | 严格按书籍权限过滤（`relationship` + 私有书籍 token）   |
| **性能**     | 搜索与全文读取限流/分页；热点缓存（书籍目录、最近更新） |
| **内容质量** | 优先 Markdown/Content，必要时 HTML→Text 转换            |

---

## 待确认事项

> [!IMPORTANT]
> 在开始实施前，请确认以下决策点：

1. **部署形态**：采用 Sidecar MCP 还是内置到 BookStack 进程？
2. **鉴权方式**：独立 MCP Token 还是复用成员登录/Token？
3. **检索增强**：是否接入 ES 做全文检索增强？

---

## 下一步

如果你认可此方案，我可以进一步：

- [ ] 把实施拆成可执行任务
- [ ] 给出详细接口定义
- [ ] 提供路由与数据库字段映射
