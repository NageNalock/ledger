# 项目文件结构与配置模板

## 顶层结构

```
ledger/
├── api/                 后端服务代码 (Gin + GORM)
├── src/                 前端应用代码 (Vue 3 + Vite)
├── public/              前端静态资源
├── dist/                前端构建产物 (build.sh 生成)
├── uploads/             上传文件目录
├── build.sh             编译脚本 (前端 + 后端)
├── server_control.sh    启动脚本 (start/stop/restart/status)
├── DEPLOY_README.md     部署说明
├── .env.example         配置模板
├── go.mod/go.sum        后端依赖
├── package.json         前端依赖
└── vite.config.ts       前端构建配置
```

## 后端结构 (`api/`)

```
api/
├── main.go              服务入口
├── router/              路由注册
├── controller/          接口实现
├── middleware/          中间件
├── db/                  数据库初始化
├── model/               数据模型
└── utils/               工具方法 (JWT 等)
```

## 前端结构 (`src/`)

```
src/
├── views/               页面视图
├── components/          通用组件
├── router/              路由配置
├── stores/              状态管理 (Pinia)
├── api/                 请求封装
├── assets/              静态资源
└── App.vue/main.ts      应用入口
```

## 服务配置模板

项目通过环境变量读取配置。

`.env.example`:

```
PORT=8080
DB_PATH=ledger.db
JWT_SECRET=your_secret_key_here
LOG_DIR=logs
LOG_MAX_SIZE_MB=50
LOG_MAX_BACKUPS=10
LOG_MAX_AGE_DAYS=14
```

### 变量说明

- `PORT`: 服务端口号
- `DB_PATH`: SQLite 数据库文件路径
- `JWT_SECRET`: JWT 签名密钥
- `LOG_DIR`: 日志输出目录
- `LOG_MAX_SIZE_MB`: 单个日志文件最大大小（MB）
- `LOG_MAX_BACKUPS`: 保留的历史日志文件数量
- `LOG_MAX_AGE_DAYS`: 日志保留天数

### 使用方式

在 Linux 服务器上可以这样启用配置：

```bash
export PORT=80
export DB_PATH=/data/ledger/ledger.db
export JWT_SECRET=replace_with_secure_value
./server_control.sh start
```
