# Python SDK - Go Server Integration Guide

本指南展示如何使用 Python `langgraph-sdk` 与 Go 版本的 LangGraph Server 协同工作。

## 状态

✅ **完全兼容** - Python SDK 可以无缝使用 Go Server 提供的所有 API。

## 支持的功能

### Assistants API
- ✅ `GET /assistants` - 列出所有 assistants
- ✅ `POST /assistants` - 创建新 assistant
- ✅ `GET /assistants/{assistant_id}` - 获取指定 assistant
- ✅ `POST /assistants/search` - 搜索 assistants
- ✅ `POST /assistants/count` - 统计 assistants 数量

### Threads API
- ✅ `GET /threads` - 列出所有 threads
- ✅ `POST /threads` - 创建新 thread
- ✅ `GET /threads/{thread_id}` - 获取指定 thread
- ✅ `PATCH /threads/{thread_id}` - 更新 thread
- ✅ `DELETE /threads/{thread_id}` - 删除 thread
- ✅ `POST /threads/search` - 搜索 threads
- ✅ `POST /threads/count` - 统计 threads 数量
- ✅ `POST /threads/{thread_id}/state` - 获取/更新 thread 状态
- ✅ `POST /threads/{thread_id}/history` - 获取 thread 历史

### Runs API
- ✅ `POST /runs` - 创建 run
- ✅ `POST /threads/{thread_id}/runs` - 在 thread 中创建 run
- ✅ `GET /threads/{thread_id}/runs` - 列出 thread 的所有 runs
- ✅ `GET /runs/{run_id}` - 获取指定 run
- ✅ `GET /threads/{thread_id}/runs/{run_id}` - 获取 thread 中的指定 run
- ✅ `DELETE /runs/{run_id}` - 删除 run
- ✅ `DELETE /threads/{thread_id}/runs/{run_id}` - 删除 thread 中的 run

### Other APIs
- ✅ `GET /health` - 健康检查
- ✅ CORS 支持
- ✅ API Key 认证

## 快速开始

### 1. 启动 Go Server

```bash
cd /path/to/langgraph-go
go run ./server/example/main.go
```

Server 将在 `http://localhost:8123` 启动。

### 2. 使用 Python SDK 连接

```python
from langgraph_sdk import get_sync_client

# 连接到 Go server
client = get_sync_client(url="http://localhost:8123")
```

### 3. 完整工作流程示例

```python
from langgraph_sdk import get_sync_client
import httpx

# 连接
client = get_sync_client(url="http://localhost:8123")

# Step 1: 列出 assistants
assistants = client.assistants.search()
print(f"Found {len(assistants)} assistants")

# Step 2: 创建 thread
thread = client.threads.create(metadata={"source": "python_sdk"})
thread_id = thread["thread_id"]

# Step 3: 如果没有 assistant，创建一个
if not assistants:
    http_client = httpx.Client(transport=httpx.HTTPTransport(retries=0))
    response = http_client.post(
        "http://localhost:8123/assistants",
        json={
            "graph_id": "simple-graph",  # 必须匹配 server 注册的 graph
            "name": "My Assistant",
            "config": {"recursion_limit": 25}
        }
    )
    assistant = response.json()
else:
    assistant = assistants[0]

# Step 4: 执行 run
run = client.runs.create(
    thread_id=thread_id,
    assistant_id=assistant["assistant_id"],
    input={"message": "Hello from Python SDK!"}
)
print(f"Run created: {run['run_id']}, Status: {run['status']}")

# Step 5: 获取 run 状态
import time
for _ in range(5):
    run = client.runs.get(thread_id, run['run_id'])
    if run['status'] in ['success', 'error']:
        break
    time.sleep(1)

print(f"Final status: {run['status']}")
print(f"Output: {run.get('output')}")

# Step 6: 列出 thread 的所有 runs
runs = client.runs.list(thread_id)
print(f"Total runs: {len(runs)}")

# Step 7: 清理
client.threads.delete(thread_id)
```

### 4. 异步客户端示例

```python
import asyncio
from langgraph_sdk import get_client

async def main():
    async with get_client(url="http://localhost:8123") as client:
        # 创建 thread
        thread = await client.threads.create()
        print(f"Created thread: {thread['thread_id']}")

        # 获取 thread
        got = await client.threads.get(thread['thread_id'])
        print(f"Got thread: {got['thread_id']}")

        # 搜索 assistants
        assistants = await client.assistants.search()
        print(f"Found {len(assistants)} assistants")

asyncio.run(main())
```

## 测试

### 运行基础兼容性测试

```bash
cd /path/to/langgraph-go/tests/integration
python3 test_compat.py
```

### 运行完整功能测试

```bash
cd /path/to/langgraph-go/tests/integration
python3 test_full.py
```

### 运行端到端演示

```bash
cd /path/to/langgraph-go/tests/integration
python3 demo.py
```

## API 响应格式

### Assistants

**GET /assistants** - 返回数组
```json
[
  {
    "assistant_id": "uuid",
    "graph_id": "graph-id",
    "name": "Assistant Name",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z",
    "config": {},
    "metadata": {},
    "version": 1
  }
]
```

**POST /assistants/search** - 返回数组
```json
[
  { ... assistant object ... }
]
```

**POST /assistants/count** - 返回整数
```json
42
```

### Threads

**GET /threads** - 返回数组
```json
[
  {
    "thread_id": "uuid",
    "status": "idle",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z",
    "config": {},
    "metadata": {}
  }
]
```

**GET /threads/{thread_id}** - 返回单个对象
```json
{ ... thread object ... }
```

### Runs

**GET /threads/{thread_id}/runs** - 返回数组
```json
[
  {
    "run_id": "uuid",
    "thread_id": "thread-uuid",
    "assistant_id": "assistant-uuid",
    "status": "success",
    "input": {},
    "output": {},
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z",
    "config": {},
    "metadata": {},
    "stream_mode": ["values", "updates"]
  }
]
```

## 重要注意事项

### httpx 配置

在某些环境下，httpx 的默认重试行为可能导致 502 错误。推荐使用自定义 transport：

```python
import httpx

# 创建无重试的 transport
transport = httpx.HTTPTransport(retries=0)
client = httpx.Client(transport=transport)
```

### Runs API 调用

- `client.runs.get()` 需要两个参数：`thread_id` 和 `run_id`
- `client.runs.list()` 只需要 `thread_id`

```python
# 获取特定 run
run = client.runs.get(thread_id, run_id)

# 列出 thread 的所有 runs
runs = client.runs.list(thread_id)
```

## 故障排查

### 连接失败

1. 确认 Go server 正在运行：
   ```bash
   curl http://localhost:8123/health
   ```

2. 检查端口是否被占用：
   ```bash
   lsof -i :8123
   ```

### 502 错误

使用自定义 httpx transport：
```python
transport = httpx.HTTPTransport(retries=0)
client = get_sync_client(url=SERVER_URL, http_client=client_with_transport)
```

### 方法不允许错误

确认使用正确的 HTTP 方法：
- 创建：POST
- 获取：GET
- 更新：PATCH
- 删除：DELETE

## 运行所有测试

```bash
# 停止旧 server
pkill -f langgraph-server

# 启动新 server
cd /Users/yingfeng/codebase/graph/langgraph-go
go build -o /tmp/langgraph-server ./server/example/main.go
/tmp/langgraph-server > /tmp/server.log 2>&1 &

# 运行测试
cd tests/integration
python3 test_compat.py      # 基础测试
python3 test_full.py         # 完整测试
python3 test_e2e.py         # 端到端测试
python3 demo.py              # 完整演示
```

## 兼容性矩阵

| API | 方法 | 状态 | 说明 |
|-----|--------|--------|------|
| Health | GET | ✅ | 完全兼容 |
| Assistants List | GET | ✅ | 返回数组格式 |
| Assistants Get | GET | ✅ | 按 ID 获取 |
| Assistants Search | POST | ✅ | 支持过滤和分页 |
| Assistants Count | POST | ✅ | 返回整数 |
| Assistants Create | POST | ✅ | 支持配置和元数据 |
| Threads List | GET | ✅ | 返回数组格式 |
| Threads Get | GET | ✅ | 按 ID 获取 |
| Threads Create | POST | ✅ | 支持配置和元数据 |
| Threads Update | PATCH | ✅ | 支持配置和元数据 |
| Threads Delete | DELETE | ✅ | 删除后返回 204 |
| Threads Search | POST | ✅ | 支持过滤和分页 |
| Threads Count | POST | ✅ | 返回整数 |
| Runs Create | POST | ✅ | 支持多种参数 |
| Runs Get | GET | ✅ | 需要 thread_id 和 run_id |
| Runs List | GET | ✅ | 按 thread_id 列出 |
| Runs Delete | DELETE | ✅ | 删除后返回 204 |

## 总结

Python `langgraph-sdk` 与 Go 版本的 LangGraph Server **完全兼容**。所有核心 API 都已实现并经过测试，可以无缝协同工作。

**关键点：**
1. ✅ 所有 CRUD 操作支持
2. ✅ 同步和异步客户端都支持
3. ✅ API 响应格式与 Python SDK 期望一致
4. ✅ 错误处理兼容
5. ✅ 测试覆盖全面

你可以放心使用 Python SDK 来控制 Go Server，构建强大的多代理应用程序！
