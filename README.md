# 工厂食堂报餐系统

依据 `doc/xuqiu.md`（需求）与 `doc/sheji.md`（设计）实现的单服务全栈系统：Go 后端 + Vue 3 双前端（H5 员工端 / Element Plus 管理端），数据库 Supabase PostgreSQL。

## 快速开始（本地）

### 1. 数据库
任选其一：
- Docker：`docker compose up -d`（本地 PostgreSQL，连接串 `postgres://canteen:canteen123@localhost:5432/canteen?sslmode=disable`）
- 直接使用 Supabase（见 `doc/sheji.md` §7.3 连接串）

### 2. 启动后端（自动迁移）
```cmd
cd server
set AUTO_MIGRATE=1
set DATABASE_URL=<连接串>
set JWT_SECRET=dev-secret
go run ./cmd/api
```
默认端口 8080；迁移幂等（schema_migrations 表记录版本）。

### 3. 前端开发模式
```cmd
pnpm install
pnpm h5     # 员工端 http://localhost:5173（代理 /api → 8080）
pnpm admin  # 管理端 http://localhost:5174
```

### 4. 构建与部署
- `pnpm build` 构建双前端；`Dockerfile` 多阶段构建并经 go:embed 内嵌到单二进制
- Render 部署：绑定 GitHub 仓库即可（`render.yaml` 就绪），`AUTO_MIGRATE=1` 启动时自动迁移

### 5. 验证
- 单元/集成测试：`cd server && go test ./...`
- 端到端冒烟（真实库）：先启动服务，再 `scripts\run-smoke.cmd`（或 `SMOKE_BASE=... go run ./cmd/smoke`）

## 初始账号

| 角色 | 工号 | 初始密码 |
|---|---|---|
| 超级管理员 | admin | admin123（首登强制改密） |
| 食堂管理员 | 超管在"账号管理"创建 | 123456 |
| 普通员工 | 管理员在"员工管理"添加/CSV 导入 | 123456 |

> 若执行过冒烟测试，admin 密码已被重置为 `Admin2026!`。

## 实现偏差说明（相对 doc/sheji.md）

| 设计 | 实现 | 原因 |
|---|---|---|
| Web 框架 Gin | Go 1.22+ 标准库 `net/http`（ServeMux 路径参数 + 中间件） | Gin 传递依赖 `golang-asm` 上游仓库已不可访问（goproxy 无法提供）；标准库功能等价 |
| sqlc 生成查询 | 手写 pgx/v5 参数化查询（`internal/store/pg`） | 保持零代码生成依赖；全部查询参数化，无拼接 |
| Excel(xlsx)+CSV 导入 | CSV 导入（模板列 `工号,姓名,部门`，UTF-8 建议 BOM） | 缩减依赖；Excel 请"另存为 CSV"。导入接口签名不变 |
| static 产物挂载 `server/static` | `server/cmd/api/static`（go:embed 路径必须在包目录内） | embed 约束；Dockerfile 已同步 |
| 迁移工具 golang-migrate | 内嵌迁移执行器（语义一致：`*.up.sql/*.down.sql` 成对、幂等、事务内执行） | 免维护依赖；文件格式兼容，如需切换 golang-migrate 无需改 SQL |

其余（数据库 DDL、API 契约、业务规则、时间规则、角色权限、错误码、部署配置）均按设计文档实现。

## 目录结构

```
canteen-system/
├── apps/h5          # 员工端 H5（Vue3+Vant4，/menus /history /my/*）
├── apps/admin       # 管理端（Vue3+Element Plus，/admin/*）
├── packages/api-client  # 共享 TS API 客户端
├── server/
│   ├── cmd/api          # 主服务（静态路由：/api/*、/admin*、h5 fallback）
│   ├── cmd/smoke        # 端到端冒烟客户端（幂等，可重复运行）
│   ├── cmd/genhash      # bcrypt 哈希生成工具
│   ├── internal/api     # 路由与处理器（35 个端点）
│   ├── internal/auth    # JWT/bcrypt/中间件/防爆破
│   ├── internal/domain  # 纯业务判定（违约推导/核销决策/点评校验链）
│   ├── internal/timewindow  # 时间规则引擎（7 条规则，Asia/Shanghai）
│   ├── internal/httpx   # 统一响应 {code,msg,data} + 错误码
│   ├── internal/store   # Store 接口 + pg 实现 + 内存实现（测试用）
│   ├── internal/migrate # 迁移执行器
│   └── migrations/      # SQL 迁移（含超管种子）
├── scripts/             # 启动/冒烟脚本
├── Dockerfile / render.yaml / docker-compose.yml
└── doc/                 # 需求/设计/任务清单
```

## 测试覆盖

- `timewindow`：7 条时间规则全部边界（00:00 截止、±15 天、30 天历史、核销截止时刻、当日限制、点评窗口）
- `domain`：违约推导状态机、核销 4 分支矩阵（含防双重违约、防提前核销）、点评校验链 ①②③、报名窗口
- `auth`：bcrypt、JWT 签发/校验/篡改、mcp 语义、防爆破锁定
- `api`（内存 Store + httptest 集成测试）：登录/锁定/首登强改密/重置密码后旧 token 失效、发布与冲突明细、报名/修改/取消状态机、核销/登记互斥全分支、点评链全拒因、建议流转、超管权限（禁超管拒绝、重置超管拒绝）、禁用即时生效、CSV 导入明细、统计口径
- `cmd/smoke`：对真实数据库的全链路冒烟（17 项断言，幂等可重复运行）
