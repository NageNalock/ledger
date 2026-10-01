---
name: ledger
description: Use an authenticated Ledger MCP service to query, create, update or delete bookkeeping transactions when the user asks to work with that Ledger instance.
---

# Ledger 记账

## 发现与连接

以用户提供的 Ledger 网站 origin（协议、主机、端口）为服务地址，MCP 端点是同一 origin 下的 `/mcp`，传输为 **Streamable HTTP**。公开说明在 `/agent`，发现索引在 `/llms.txt`；文档无需登录，工具必须鉴权。

使用用户提供的 **Ledger 网页账号密码**，每次 HTTP 请求都携带：

```http
Authorization: Basic <base64(UTF-8编码的用户名:密码)>
Content-Type: application/json
Accept: application/json, text/event-stream
```

无需先登录网页、获取 JWT、验证码或 OAuth。HTTP Basic 仍然是鉴权，不代表允许匿名调用。不要使用服务器 SSH/root 密码代替 Ledger 账号。凭据不完整时向用户索取缺少的信息；不要猜测、重置或绕过认证。不要把真实凭据写进本 skill、URL、工具参数、仓库或日志。

通过支持自定义 Authorization 请求头的 MCP 客户端连接：初始化后读取 `tools/list` 获得当前 schema，再调用工具。不支持 MCP 客户端时也可以发送 JSON-RPC HTTP POST：

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"ledger-agent","version":"1.0"}}}
```

初始化后发送 `{"jsonrpc":"2.0","method":"notifications/initialized"}`（无需 id），再发送 `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`。后续 POST 继续携带相同 Basic 头和协商的 `MCP-Protocol-Version`。服务无状态，不需要保存 session id。

Basic 的 Base64 不是加密，发送实际密码时使用 HTTPS 或用户已配置的 SSH 隧道。隧道转发到服务监听端口后，MCP URL 改为本地转发端口的 `/mcp`；Ledger 账密仍然必需。

Codex 本地配置可使用环境变量传递请求头；`LEDGER_MCP_AUTH` 的值是完整的 `Basic ...` 字符串：

```toml
[mcp_servers.ledger]
url = "https://YOUR_LEDGER_ORIGIN/mcp"
env_http_headers = { Authorization = "LEDGER_MCP_AUTH" }
```

将 URL 替换为用户指定的实例或 SSH 隧道地址。配置仅供接入时使用，不擅自修改用户的其他客户端配置。环境变量需要对运行客户端的进程可见。

## 记账工作流

处理抖音商城或生活服务账单截图时，先读取同一 origin 的 [抖音截图记账 Skill](/skills/douyin-ledger/SKILL.md)：必须有用户明确选择的消费时间或入账时间口径，结合金额、商品及两种日期核对已有流水后再记账。

- 查询：用 `list_transactions` 按日期、分类、项目、来源筛选；分页从 1 开始，每页最多 100 条，遍历分页后再声称已查全。单笔查询用 `get_transaction`。
- 新增：先用 `list_categories` 解析一级、二级分类 ID；需要项目时用 `list_projects`。通过 `create_transaction` 写入用户要求的金额、时间、商户和备注。金额单位为元，正数为支出、负数为退款，不能为 0；时间使用含明确时区的 RFC3339。分类必须存在且父子关系匹配，不能猜 ID。
- 修改：先读取原流水；`update_transaction` 是完整替换可编辑字段，保留用户未要求修改的字段。用空字符串清空备注，用 `project_id: null` 解除项目。不要把“修改备注”变成清空其他字段。
- 删除：用 `delete_transaction` 删除用户指定的流水。匹配到多条且范围不明确时先澄清，不扩大删除范围。
- 写入响应成功后报告流水 ID 及变更结果。`isError: true` 是业务失败。创建操作遇到超时或结果不明时先查询是否已写入，再决定是否重试，避免重复记账。

普通账号通过 MCP 仅能访问自己的流水，管理员可访问全部；新流水归属于当前账号。分类和项目是共享字典。MCP 不提供账号管理、系统密钥读取或整库导出工具。

401 表示账密无效或缺失，停止重试并检查凭据；改密、删号后旧凭据立即失效。403 可能是携带了浏览器 Origin（该端点供 MCP 客户端调用，不支持浏览器跨域 fetch）。429 时按 `Retry-After` 等待，禁止连续重试。当前限额为每个 TCP 来源 IP 每分钟 120 次请求。
