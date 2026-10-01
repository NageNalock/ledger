# 数据迁移与 MCP 记账

## 完整迁移备份

管理员登录网页后，打开 **设置 → 数据迁移 → 导出迁移备份**。
接口为 `GET /api/backup/export`，使用现有网页登录 Bearer token；普通用户返回 403。

ZIP 包包含：

- `ledger.db`：完整 SQLite 快照，包括所有用户及密码哈希、流水、分类、项目、草稿、预算、AI 分析和系统设置。
- `uploads/`：上传文件。
- `manifest.json`：格式版本、时间及文件 SHA-256 校验和。

使用 SQLite `VACUUM INTO` 生成一致性快照，兼容 WAL。临时备份位于私有临时目录，下载结束后清除，禁止 HTTP 缓存。上传目录若包含符号链接或非普通文件会拒绝导出。

**备份含账号密码哈希和系统中保存的 AI 密钥，按敏感数据保管。** `.env`、日志、源代码和编译产物不在备份内。SHA-256 用于检测文件损坏，不是备份真实性签名，只恢复可信来源的包。

### 迁移到另一台机器

1. 在新机器部署含此功能的项目版本（Go 1.25+、Node 18+，执行 `npm ci` 和 `./build.sh`）。新实例先不要启动。
2. 暂停旧系统的记账、导入和其他写入操作，等待进行中的操作完成，再导出最终备份。快照完成后的新增数据不会包含在包内；迁移切换期间不要继续使用旧实例写入。
3. 将备份通过 SSH/SCP 等加密方式传到新机器，恢复到一个**不存在的新目录**：

   ```bash
   python3 scripts/restore_backup.py /path/to/ledger-backup.zip /srv/ledger-data
   ```

   脚本校验格式、文件列表、路径、SHA-256 及 SQLite 完整性，拒绝覆盖已有目录；解压上限为 20 GiB。
4. 将恢复出的 `uploads` 目录放入新项目的工作目录（若已存在同名目录，先另行保留，避免覆盖）：

   ```bash
   mv /srv/ledger-data/uploads /path/to/new-ledger/uploads
   ```

5. 在新项目中创建 `.env`，设置 `DB_PATH=/srv/ledger-data/ledger.db`、新的 `JWT_SECRET`（至少 32 字符）及所需的 `PORT`、`CORS_ALLOW_ORIGINS` 等配置。`SEED_ADMIN=0`，保留备份中的账号。启动前加载配置：

   ```bash
   set -a
   source .env
   set +a
   ./server_control.sh start
   ```

6. 使用原记账账号密码登录，检查流水、预算及附件，再切换访问地址。新 JWT 密钥会使旧网页登录 token 失效，需要重新登录；原账号密码保持有效。旧机器的数据保留到迁移核验完成。

## MCP 接口

地址：`/mcp`，传输：Streamable HTTP，无状态，使用官方 Go MCP SDK。

每次请求必须携带：

```http
Authorization: Basic <base64(记账系统用户名:记账系统密码)>
```

这是 **Ledger 网页账号**，不是服务器的 root / SSH 密码。无需验证码或预先获取 token；服务器每次使用数据库中的 bcrypt 哈希验证，改密或删除账号后原凭据立即失效。不接受 URL query 或工具参数中的密码。未认证/错误账密返回 HTTP 401。

| 工具 | 功能 |
| --- | --- |
| `list_transactions` | 分页查询，支持日期、分类、项目、来源和管理员用户筛选 |
| `get_transaction` | 按 ID 查询单条 |
| `create_transaction` | 新增，归属当前账号 |
| `update_transaction` | 完整替换可编辑字段，先查询并保留未修改字段 |
| `delete_transaction` | 按 ID 永久删除 |
| `list_categories` | 查询共享分类及其 ID |
| `list_projects` | 查询共享项目及其 ID |

普通用户通过 MCP 只能读写自己的流水；管理员可以读写所有流水，但新建流水仍归属于管理员本人。分类、项目是共享字典。已有网页共享账本的权限语义保持不变，MCP 权限范围不代表网页所有接口已改成私有账本。

时间建议使用含时区的 RFC3339，如 `2026-09-14T12:00:00+08:00`。金额单位为元，正数表示支出，负数表示退款，不能为 0。一级、二级分类必须匹配；项目必须存在。每页最多 100 条。修改是完整替换，可用空字符串清空备注、`project_id: null` 清除项目。

创建操作在网络结果不确定时不要直接重试，先查询确认是否已写入，以免重复记账。返回 `isError: true` 表示业务失败，不能当作已成功完成。

接口拒绝带 Origin 的浏览器请求。按 TCP 来源 IP 限制每分钟 120 次请求（包括鉴权成功请求），超限返回 429 和 `Retry-After: 60`；不信任可伪造的转发 IP 头。反向代理后多个客户端会共享代理 IP 的额度。

### Codex 接入

账号密码的 Basic 编码不是加密；使用 HTTPS，或通过 SSH 隧道连接。当前服务器只有 HTTP 时，可在本地保持以下隧道运行：

```bash
ssh -N -L 18080:127.0.0.1:1125 root@43.155.134.182
```

在启动 Codex 的环境中设置 `LEDGER_MCP_AUTH`，其值为 `Basic ` 加账密 Base64（不要使用 SSH 密码）：

```bash
export LEDGER_MCP_AUTH="$(python3 - <<'PY'
import base64, getpass
username = getpass.getpass('Ledger 用户名: ')
password = getpass.getpass('Ledger 密码: ')
print('Basic ' + base64.b64encode((username + ':' + password).encode()).decode())
PY
)"
```

然后在 Codex `config.toml` 中添加（桌面应用必须继承这个环境变量；可在 MCP 设置中改用对应的静态 Authorization header）：

```toml
[mcp_servers.ledger]
url = "http://127.0.0.1:18080/mcp"
env_http_headers = { Authorization = "LEDGER_MCP_AUTH" }
```

使用 HTTPS 域名时将 `url` 改为 `https://你的域名/mcp`，不需要 SSH 隧道。

重启 MCP 连接后，可以说：“记录今天午餐 25 元，分类为餐饮/午餐”，或“查询本月流水”。Codex 会调用工具查询分类 ID 并记账。此方式使用自定义 Basic 请求头，不使用 OAuth 登录。

参考：[Codex 官方 MCP 配置](https://developers.openai.com/codex/mcp)、[官方 Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk)。

## 验证

```bash
go test ./api/...
python3 -m unittest discover -s scripts -p 'test_*.py'
npm run build
```

集成测试使用临时 SQLite 数据库和真实 HTTP MCP 客户端，覆盖鉴权、账号间隔离、管理员操作、CRUD、分类/项目校验、改密失效和迁移备份内容。Python 测试覆盖恢复、禁止覆盖、损坏文件与目录穿越。
