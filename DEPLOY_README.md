# Ledger 部署指南

本项目已拆分为独立的编译脚本与启动脚本。

## 编译 (Linux)

在服务器或部署机器上执行：

```bash
chmod +x build.sh
./build.sh
```

编译产物：

- 前端静态资源：`dist/`
- 后端可执行文件：`ledger-server`

## 启动与管理

启动脚本：`server_control.sh`

```bash
chmod +x server_control.sh
./server_control.sh start
```

常用命令：

```bash
./server_control.sh status
./server_control.sh stop
./server_control.sh restart
```

服务默认监听 `:8080`，访问地址：`http://服务器IP:8080`

## 数据说明

本项目使用 SQLite，数据库文件为 `ledger.db`，不会被编译脚本打包或复制。
请自行备份 `ledger.db` 与 `uploads/` 目录。
