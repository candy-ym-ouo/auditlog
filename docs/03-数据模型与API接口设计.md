# 03 · 数据模型与 API 接口设计

> 本文档定义持久化表结构（DDL 为设计描述）与全部 REST 端点规格，是编码时数据层与接口层的实现依据。

---

## 目录

1. 数据模型总览
2. 表结构（DDL 设计）
3. 索引与约束
4. API 通用约定
5. 端点规格（13 个）
6. 错误码表
7. 典型调用示例

---

## 1. 数据模型总览

```
meta（键值对）              audit_entries（主库链）       archive_batches（归档清单）
┌──────────────┐           ┌──────────────────────┐     ┌──────────────────────┐
│ key | value  │           │ id | seq | prev_hash │     │ batch_no | start_seq │
│ head_seq     │◄──────────│ hash | actor | action│     │ end_seq | prev_hash  │
│ head_hash    │           │ target | detail      │     │ head_hash | count    │
│ …            │           │ event_time           │     │ payload_hash | time  │
└──────────────┘           └──────────────────────┘     └──────────────────────┘
                                  │ 归档时搬移 ↓              │
                           archive_entries（归档库，保留原字段）
                           ┌──────────────────────┐
                           │ 同 audit_entries 结构 │
                           └──────────────────────┘
```

- `meta`：链头游标、最近归档时间等运行状态；
- `audit_entries`：活跃链（主库），追加为主；
- `archive_batches`：归档批次清单（manifest），衔接链的中间段；
- `archive_entries`：已归档条目，字段与主库一致，保留原始 `seq/prev_hash/hash`。

## 2. 表结构（DDL 设计）

> 以下 SQL 为**设计文档**，实际迁移由 `internal/store/migrate.go` 按版本执行（docs/01 §8.4）。

### 2.1 v1：基础表

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_entries (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    seq         INTEGER NOT NULL UNIQUE,
    prev_hash   TEXT NOT NULL,               -- 64 位 hex；创世为全零
    hash        TEXT NOT NULL,               -- 64 位 hex
    actor       TEXT NOT NULL,
    action      TEXT NOT NULL,
    target      TEXT NOT NULL,
    detail      TEXT NOT NULL,               -- 规范化 JSON 字符串
    event_time  TEXT NOT NULL                -- RFC3339Nano (UTC)
);
```

### 2.2 v2：归档表

```sql
CREATE TABLE IF NOT EXISTS archive_batches (
    batch_no     INTEGER PRIMARY KEY,
    start_seq    INTEGER NOT NULL,
    end_seq      INTEGER NOT NULL,
    prev_hash    TEXT NOT NULL,              -- 区间前一条 hash（衔接上一段）
    head_hash    TEXT NOT NULL,              -- 区间最后一条 hash（衔接下一段）
    item_count   INTEGER NOT NULL,
    payload_hash TEXT NOT NULL,              -- 批次载荷摘要
    archived_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS archive_entries (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    seq         INTEGER NOT NULL UNIQUE,
    prev_hash   TEXT NOT NULL,
    hash        TEXT NOT NULL,
    actor       TEXT NOT NULL,
    action      TEXT NOT NULL,
    target      TEXT NOT NULL,
    detail      TEXT NOT NULL,
    event_time  TEXT NOT NULL,
    batch_no    INTEGER NOT NULL             -- 归属批次
);
```

### 2.3 v3：追溯索引

```sql
CREATE INDEX IF NOT EXISTS idx_entries_actor  ON audit_entries(actor);
CREATE INDEX IF NOT EXISTS idx_entries_action ON audit_entries(action);
CREATE INDEX IF NOT EXISTS idx_entries_time   ON audit_entries(event_time);
CREATE INDEX IF NOT EXISTS idx_arch_entries_batch ON archive_entries(batch_no);
```

## 3. 索引与约束

| 约束/索引 | 对象 | 目的 |
| --- | --- | --- |
| `seq UNIQUE` | 主库 + 归档库 | 链序唯一，验证合并排序可靠 |
| `idx_entries_actor/action/time` | 主库 | 追溯查询加速 |
| `idx_arch_entries_batch` | 归档库 | 批次导出加速 |
| 禁止 UPDATE/DELETE | 审计表 | 不可变性（应用层保证 + 文档约定） |
| 外键 | 不启用 | 保持 SQLite 简单性，一致性由服务事务保证 |

## 4. API 通用约定

- **Base**：`/api/v1`
- **内容类型**：请求与响应均为 `application/json; charset=utf-8`
- **认证**（可选启用）：`Authorization: Bearer <token>`
- **时间格式**：RFC3339Nano（UTC），如 `2025-01-01T08:00:00.123Z`
- **分页**：`page`（≥1，默认 1）、`page_size`（1–200，默认 20）；响应 `{items, page, page_size, total}`
- **错误体**：`{"error": {"code": "…", "message": "…"}}`

## 5. 端点规格（13 个）

### 5.1 GET /api/v1/health

健康检查（含数据库连通性）。

```json
// 200
{"status":"ok","version":"1.0.0","db":"ok","time":"2025-01-01T00:00:00Z"}
```

### 5.2 GET /api/v1/stats

运行统计，供前端总览面板。

```json
// 200
{
  "total_entries": 12500, "archived_entries": 11500, "active_entries": 1000,
  "head_seq": 12500, "head_hash": "9f86d081884c7d659a2feaa0c55ad015…",
  "batch_count": 3, "last_archive_at": "2025-01-01T00:00:00Z",
  "last_verify_at": "2025-01-01T00:00:00Z", "last_verify_status": "ok"
}
```

### 5.3 POST /api/v1/entries

追加审计条目（核心写入）。

请求：

```json
{
  "actor": "alice",
  "action": "perm_change",
  "target": "role:ops",
  "detail": {"role": "ops", "granted": true},
  "event_time": "2025-01-01T08:00:00Z"   // 可选，缺省用服务端时间
}
```

响应（201）：

```json
{
  "id": 952, "seq": 12501, "prev_hash": "9f86…", "hash": "ab12…",
  "actor": "alice", "action": "perm_change", "target": "role:ops",
  "detail": {"granted": true, "role": "ops"},
  "event_time": "2025-01-01T08:00:00Z"
}
```

错误：`400 invalid_request`（字段缺失/含分隔符/时间非法）、`401 unauthorized`、`409 chain_error`（链状态异常，拒绝写入）。

### 5.4 GET /api/v1/entries

条件查询 + 分页。查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `actor` | string | 操作人精确匹配 |
| `action` | string | 动作精确匹配 |
| `target` | string | 目标模糊匹配 |
| `from` / `to` | RFC3339 | event_time 区间 |
| `seq_from` / `seq_to` | int | 链序号区间 |
| `include_archived` | bool | 合并归档条目，默认 true |
| `page` / `page_size` | int | 分页 |

```json
// 200
{"items":[{"id":952,"seq":12501,"hash":"ab12…", …}],"page":1,"page_size":20,"total":42}
```

### 5.5 GET /api/v1/entries/head

获取链头。

```json
// 200
{"seq":12501,"hash":"ab12…","prev_hash":"9f86…","updated_at":"2025-01-01T08:00:01Z"}
```

### 5.6 GET /api/v1/entries/:id/context?radius=5

上下文回溯：以目标条目为中心的前后各 `radius` 条（默认 5）。

```json
// 200
{
  "center": {"id": 952, "seq": 12501, "hash": "ab12…", "actor": "alice", …},
  "before": [ …5 条… ],
  "after":  [ …5 条… ],
  "link_ok": true,
  "chain_position": {"seq": 12501, "prev_hash_ok": true, "hash_ok": true}
}
```

### 5.7 POST /api/v1/verify

发起验证。请求体：

```json
{"mode": "full", "start_seq": 1, "end_seq": 0, "sample_step": 100}
```

- `mode`：`full`（默认）| `range` | `head` | `spot`；
- `range` 需要 `start_seq/end_seq`；`spot` 需要 `sample_step`。

响应（200，验证报告，见 docs/01 §5.3）：

```json
{
  "mode": "full", "status": "ok",
  "checked_entries": 12501, "start_seq": 1, "end_seq": 12501,
  "break_seq": 0, "break_reason": "",
  "head_hash": "ab12…",
  "archives_checked": 3,
  "archives": [
    {"batch_no": 1, "start_seq": 1, "end_seq": 8000, "head_hash": "c3f2…", "linked": true}
  ],
  "duration_ms": 86, "verified_at": "2025-01-01T08:10:00Z"
}
```

状态取值：`ok` | `hash_mismatch` | `link_broken` | `head_mismatch`。

### 5.8 POST /api/v1/archive

手动触发归档（异步执行）。

```json
// 200（任务已受理）
{"accepted": true, "note": "archive task enqueued"}
```

### 5.9 GET /api/v1/archive/status

后台归档任务状态。

```json
// 200
{
  "running": false, "last_run_at": "2025-01-01T07:00:00Z",
  "last_batch_no": 3, "last_error": "", "last_status": "ok",
  "threshold": 10000, "keep_min": 1000, "max_age_days": 90
}
```

`last_status` 取值：`ok`（本次写入批次）、`noop`（本次无可归档数据，运行正常）、`canceled`（运行被取消，未提交批次，`last_batch_no` 保留上次成功值）、`error`（存储返回非取消类错误，详见 `last_error`）。

### 5.10 GET /api/v1/archive/batches

归档批次列表（分页）。

```json
// 200
{"items":[{"batch_no":3,"start_seq":8001,"end_seq":12500,"item_count":4500,
           "head_hash":"ab12…","archived_at":"2025-01-01T07:00:00Z"}], …}
```

### 5.11 GET /api/v1/archive/batches/:no

批次详情 / 导出（含清单与全部条目，离线可重放）。

```json
// 200
{
  "batch": {"batch_no": 3, "start_seq": 8001, "end_seq": 12500,
            "prev_hash": "c3f2…", "head_hash": "ab12…",
            "item_count": 4500, "payload_hash": "d4e5…",
            "archived_at": "2025-01-01T07:00:00Z"},
  "entries": [ …同 audit_entries 结构… ]
}
```

### 5.12 GET /api/v1/trace/report

追溯报告导出（查询参数同 5.4，另支持 `radius`）。响应见 docs/01 §7.3。

### 5.13 静态前端

`GET /` 与 `/assets/*`：返回内嵌的 `web/index.html`、`style.css`、`app.js`（`embed.FS`）。

## 6. 错误码表

| HTTP | code | 含义 |
| --- | --- | --- |
| 400 | `invalid_request` | 参数缺失/格式错误/含分隔符 |
| 401 | `unauthorized` | 令牌缺失或错误（启用认证时） |
| 404 | `not_found` | 条目/批次不存在 |
| 409 | `chain_error` | 链状态异常，拒绝写入或验证失败 |
| 409 | `archive_busy` | 归档任务运行中，重复触发被忽略 |
| 500 | `internal_error` | 内部错误（存储/迁移失败等） |

## 7. 典型调用示例（curl，设计示意）

```bash
# 1. 健康检查
curl -s http://127.0.0.1:8080/api/v1/health

# 2. 写入审计条目
curl -s -X POST http://127.0.0.1:8080/api/v1/entries \
  -H 'Content-Type: application/json' \
  -d '{"actor":"alice","action":"login","target":"user:alice","detail":{}}'

# 3. 全链验证
curl -s -X POST http://127.0.0.1:8080/api/v1/verify \
  -H 'Content-Type: application/json' -d '{"mode":"full"}'

# 4. 追溯：alice 的所有操作
curl -s 'http://127.0.0.1:8080/api/v1/entries?actor=alice&page=1&page_size=50'

# 5. 手动归档
curl -s -X POST http://127.0.0.1:8080/api/v1/archive

# 6. 批次导出
curl -s http://127.0.0.1:8080/api/v1/archive/batches/3
```
