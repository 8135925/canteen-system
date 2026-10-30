# 工厂食堂报餐系统开发任务清单

> 依据 `doc/sheji.md`（含 7 轮评审修复后的冻结版）编制。按里程碑 M0~M7 组织，任务粒度到可独立提交/验证。每项完成后勾选 `[x]`。

## 总览

| 里程碑 | 内容 | 前置 |
|---|---|---|
| M0 | 仓库脚手架与构建链 | 无 |
| M1 | 数据库迁移 + 认证 | M0 |
| M2 | 菜谱发布/查询 + 员工端首页 + 员工管理 | M1 |
| M3 | 报名/取消/用餐记录 | M2 |
| M4 | 核销 + 未报名登记 + 违约推导 | M3 |
| M5 | 点评 + 建议 | M4 |
| M6 | 管理端统计 + 超管账号管理/看板 | M5 |
| M7 | 部署上线 | M1~M6 |

---

## M0 仓库脚手架与构建链

- [x] **T0.1 初始化 monorepo**
  - 根 `package.json`（声明 `packageManager` 字段固定 pnpm 版本）、`pnpm-workspace.yaml`
  - 目录结构对齐 sheji.md §3：`apps/h5`、`apps/admin`、`packages/api-client`、`server/`
- [x] **T0.2 Go 服务骨架**
  - `server/go.mod`（Go 1.22+，Gin v1.10、pgx/v5、golang-jwt/v5、bcrypt、validator）
  - `server/cmd/api/main.go`：监听 `0.0.0.0:$PORT`（本地缺省 8080）
  - `server/internal/config/`：加载 `DATABASE_URL`、`JWT_SECRET`、`TZ`
  - `server/static/.gitkeep` 占位（go:embed `all:` 前缀）
- [x] **T0.3 H5 前端骨架**（`apps/h5`：Vue 3 + TS + Vite + Vant 4，PC 限宽 480px 居中）
- [x] **T0.4 管理端骨架**（`apps/admin`：Vue 3 + TS + Vite + Element Plus，Vite `base: '/admin/'`，AdminLayout 左侧菜单 + 顶栏）
- [x] **T0.5 共享 api-client**（`packages/api-client`：TS 类型 + `request()` 封装，自动带 JWT、401 跳登录、`MUST_CHANGE_PASSWORD` 跳改密、统一错误 toast）
- [x] **T0.6 Dockerfile 多阶段构建**（node 构建前端 → go 构建内嵌 → alpine 运行，对齐 sheji.md §7.1；依赖分层缓存：先 COPY `package.json`/`pnpm-lock.yaml` 再 `pnpm install`、先 COPY `go.mod`/`go.sum` 再 `go mod download`，后 COPY 源码——避免改代码触发全量重下依赖；验证本地 `docker build` 通过）
- [x] **T0.6a 静态路由策略**（Go 内实现，对齐 sheji.md §7.2：`/api/*` 优先走 API；`/admin*` 返回 admin SPA；其余返回 h5 SPA；均带 SPA fallback 到 index.html——支撑 M0 验收的占位页访问）
- [x] **T0.7 本地开发环境**
  - `docker-compose.yml` 或说明文档：本地 PostgreSQL + 迁移执行方式
  - 健康检查端点 `GET /api/v1/healthz` 可访问

**验收**：`docker build` 成功；访问 `http://localhost:8080/api/v1/healthz` 返回 200；`/` 返回 h5 占位页、`/admin` 返回 admin 占位页。

---

## M1 数据库迁移 + 认证

### 数据库

- [x] **T1.1 枚举与建表迁移**（golang-migrate，对齐 sheji.md §4.2 DDL）
  - `user_role`、`meal_type` 枚举
  - `users`（含 `must_change_password`、status check、索引）
  - `menu_items`（含 `updated_at`、`unique(menu_date, meal, dish_name)`、索引）
  - `verify_rules`（含 `updated_at`，仅存覆盖值）
  - `meal_signups`（三态 status、**不设** `(user_id, menu_date, meal)` 唯一约束、两条索引）
  - `verifications`（`signup_id` unique）
  - `walk_in_records`（`unique(user_id, menu_date, meal)`）
  - `dish_reviews`（`unique(user_id, menu_date, meal, dish_name)`、rating check）
  - `suggestions`（status check）
  - 所有 comment on 语句随迁移写入
  - 每个迁移 up/down 成对编写（golang-migrate 规范），down 为 drop 反向操作
- [x] **T1.2 超管种子数据**（bcrypt 哈希替换 `<bcrypt(admin123)>` 占位符；幂等，勿重复插入）
- [x] **T1.3 sqlc 配置与查询文件骨架**（`server/internal/db/`，pgx/v5 驱动）

### 认证

- [x] **T1.4 登录接口** `POST /api/v1/auth/login`
  - 工号 + bcrypt 校验（cost=10，对齐 sheji.md §8）；禁用账号拒绝
  - JWT claims：`uid`、`role`、`mcp`
  - `remember=true` → 30 天（前端 localStorage）；否则 12 小时（sessionStorage）
- [x] **T1.5 防爆破**：`emp_no+IP` 失败计数，5 次锁 10 分钟（内存实现）
- [x] **T1.6 认证中间件**
  - token 解析与角色注入
  - 校验 `users.status`（可缓存 60s），禁用返回 401
  - **比对 DB `must_change_password` 与 token `mcp` claim**，不一致以 DB 为准返回 `403 MUST_CHANGE_PASSWORD`（防密码重置后旧 token 绕过）
- [x] **T1.7 首登强制改密拦截**：`mcp=true` 时仅放行 `PUT /my/password`、`GET /me`
- [x] **T1.8 角色中间件**：`RequireRole(employee / canteen_admin / super_admin)` 路由级挂载
- [x] **T1.9 `GET /me` / `PUT /my/password`**（全角色通用；改密成功重签 `mcp=false` token，置 `must_change_password=false`）
- [x] **T1.10 统一响应与错误码**：`{code, msg, data}`；错误码枚举 `MUST_CHANGE_PASSWORD` 等（后续里程碑递增补充）
- [x] **T1.11 slog 日志中间件**：request_id、uid、路径、耗时；不打密码明文

### 前端

- [x] **T1.12 H5 登录页**（工号+密码+记住登录）+ `/my/password` 改密页（首登强制跳转）
- [x] **T1.13 admin 登录页** + `/password` 改密页；路由守卫 `beforeEach` 校验 token 与角色，`mcp` 强制跳改密
- [x] **T1.14 api-client 对接认证接口**（login/me/password 类型与封装；token 过期不做 refresh，由前端静默重登录或下次登录完成续期，对齐 sheji.md §5.2）
- [x] **T1.15 自动登录恢复**：两端应用启动时从 localStorage 恢复 remember token → 调 `GET /me` 校验有效性（失效则清除并视为未登录），实现"下次打开自动登录"（对齐 xuqiu.md 5.1）

**验收**：admin/admin123 首登被强制改密；错误密码 5 次锁定 10 分钟；改密前签发的旧 token（mcp=true）因与 DB `must_change_password=false` 不一致被拦截返回 `MUST_CHANGE_PASSWORD`；三角色登录后各自跳转正确入口；勾选记住登录后重新打开应用自动登录（token 恢复 + `/me` 校验通过）。

---

## M2 菜谱发布/查询 + 员工端首页

### 后端

- [x] **T2.1 时间规则引擎**（单点实现，对齐 sheji.md §5.1）
  - `now < menu_date 00:00`（Asia/Shanghai）
  - 发布/可见范围 `today ≤ menu_date ≤ today+14`
  - 历史范围 `menu_date ≥ today-30`
  - 核销截止：`verify_rules` 覆盖值 || 默认 09:00/13:30/19:00
  - 点评窗口 00:00~23:59:59
  - 核销/登记目标日期必须为当日
  - **单元测试覆盖全部边界**：日期类（00:00 整点、+14/+15 天、-30/-31 天）；核销截止类（默认时刻 09:00/13:30/19:00 与 verify_rules 覆盖值，`now == 截止` 临界点）；点评窗口类（23:59:59 与次日 00:00）——对齐 sheji.md §5.1 全部 7 条规则
- [x] **T2.2 发布菜谱** `POST /admin/menus`
  - 单事务逐行插入；任一行 `menu_date` 越界（∉ [today, today+14]）→ 整批拒绝
  - 同餐同名冲突行跳过并在响应中返回失败明细，其余行提交（与员工导入模式一致，对齐 sheji.md §5.3）
- [x] **T2.3 复制上周模板** `GET /admin/menus/copy-last-week`（按星期几+餐次组织，不绑定目标日期）
- [x] **T2.4 菜谱查询** `GET /menus?start=&end=`
  - 未来 ≤15 天 / 历史 ≤30 天，越界自动截断
  - 返回含已下架菜品（附 status）+ 每菜赞踩数（按 `dish_name` 合并，供 M5 复用聚合）
  - 无需认证
- [x] **T2.5 单日聚合** `GET /menus/summary?date=`（单日刷新/深链备用）
- [x] **T2.6 菜品下架/恢复** `PATCH /admin/menu-items/:id/shelf`（更新 `updated_at`）

### 前端

- [x] **T2.7 H5 首页菜谱页**（`/menus`）
  - 日期横滑 Tab（15 天）+ 早/午/晚 Tab
  - DishCard：菜名、口味标签、赞/踩数、已下架置灰标识
  - 首屏 `GET /menus?start=<今日>&end=<今日+14>` 一次拉取
- [x] **T2.8 H5 历史菜谱页**（`/history`：日期范围 ≤30 天 + 快捷"上周"）
- [x] **T2.9 admin 菜谱发布页**（`/menus/publish`：周视图 + 菜品行编辑 + 复制上周）
- [x] **T2.10 admin 菜品下架页**（`/menus/items`：按日期餐次列出，下架/恢复开关）
- [x] **T2.11 Tabbar**（菜谱 | 记录 | 我的；M2 仅"菜谱"可达，"记录/我的"入口 M3 交付后启用，避免 M2 点击落空）

### 员工管理（提前至此：M3 起报名测试需要员工账号）

- [x] **T2.12 Excel/CSV 员工导入** `POST /admin/employees/import`
  - excelize + CSV（UTF-8 建议 BOM）；逐行校验工号非空且不重复（文件内+库内）
  - 响应 `{success_count, failures:[{row, reason}]}`；成功行即时生效（默认密码 123456、`must_change_password=true`，对齐 sheji.md §5.5）
- [x] **T2.13 员工管理接口与页面**：`GET/POST /admin/employees`、`PATCH /admin/employees/:id`（仅姓名/部门，工号不可改）+ admin 员工管理页（手动添加、编辑、导入结果明细弹窗）

**验收**：游客不登录可看 15 天菜谱与历史；发布超过 15 天的日期整批被拒；同餐同名重复发布返回失败明细且其余行成功；下架菜品 H5 显示"已下架"标识且不影响该餐次（M3 报名验证）；导入含重复工号的文件返回精确行号明细。

---

## M3 报名/取消/用餐记录

> 前置数据：员工账号已由 M2 员工管理导入/添加（默认密码 123456，首登改密）。

- [x] **T3.1 报名/修改** `POST /signups {menu_date, meal}`
  - 事务 + `SELECT ... FOR UPDATE` 锁该用户最新报餐行
  - 校验：菜谱已发布（该日期+餐次存在任意 menu_items 记录，含全部下架）、`now < menu_date 00:00`
  - 修改 = 旧行置 `cancelled`+`cancelled_at` + 新行 `signed`
  - 错误码 `SIGNUP_CLOSED`
- [x] **T3.2 取消** `DELETE /signups?menu_date=&meal=`（同时窗校验，最新行置 cancelled）
- [x] **T3.3 用餐记录** `GET /my/signups?from=&to=`
  - 报餐记录（含推导违约状态）+ 未报名就餐记录合并返回
  - 有效状态取 `signed_at` 最新一行；`signed` 且过核销截止 → 推导为违约
- [x] **T3.4 违约次数** `GET /my/violation-count`（推导违约数 + walk_in 数）
- [x] **T3.5 H5 报名交互**：餐次维度报名条（报名/已报名/已锁定三态）；已登录时调 `/my/signups` 合并状态
- [x] **T3.6 H5 用餐记录页**（`/my/records`：记录列表 + 违约次数卡片）
- [x] **T3.7 H5 个人中心页**（`/my`：用户信息展示 + 记录/建议/改密功能入口，对齐 sheji.md §6.1 路由表；Tabbar 第三项"我的"落地页）

**验收**：当日 00:00 前可报/改/取消，00:00 后三操作均拒绝；菜品全部下架仍可报名；并发报名无脏数据（历史行保留，最新行有效）；Tabbar 三项（菜谱/记录/我的）页面齐备可用。

---

## M4 核销 + 未报名登记 + 违约推导

- [x] **T4.1 核销前查询** `GET /admin/verify/lookup?emp_no=&menu_date=&meal=`
  - `menu_date` 须为当日（拒绝未来/历史日期）
  - 返回该员工当餐最新报餐行状态与可执行操作（核销 / 登记未报名 / 已违约 / 已核销）
- [x] **T4.2 执行核销** `POST /admin/verify`
  - 分支逻辑严格对齐 sheji.md §5.3：仅 `signed` 且未过截止可核销（UPDATE verified + INSERT verifications）；`signed` 且过截止 → 拒绝（已判违约，防双重违约）；`verified` → `ALREADY_VERIFIED`
  - 错误码 `VERIFY_CLOSED`
- [x] **T4.3 未报名登记** `POST /admin/walk-in`
  - `menu_date` 须为当日；仅最新行 `cancelled` 或无记录时可登记
  - `ON CONFLICT` 幂等；`signed` 过截止的员工同样拒绝（防双重违约）
  - **不受核销截止时间限制**（仅需当日）
- [x] **T4.4 核销规则管理** `GET/PUT /admin/verify-rules`
  - 默认值占位展示（09:00/13:30/19:00）；覆盖保存更新 `updated_at`
  - 注：规则调整即时生效于违约推导（含当日），接口可拒绝修改"截止时间已过"的餐次
- [x] **T4.5 admin 现场核销页**（`/verify`：工号输入自动聚焦 → 当餐记录 → 核销/登记按钮，按 lookup 结果动态显示）
- [x] **T4.6 admin 核销规则页**（`/verify-rules`）

**验收**：互斥关系全覆盖——signed 未过截止→仅可核销；signed 过截止→两操作均拒；无记录/cancelled→仅可登记；verified→提示已核销。同一员工同一餐次最终只有一条就餐记录。核销截止后 walk-in 登记仍可进行。

---

## M5 点评 + 建议

- [x] **T5.1 点评写入** `POST /reviews {menu_date, meal, dish_name, rating}`
  - 校验链：① 当日 00:00~23:59:59 ② 该员工该餐 verified 或 walk_in 存在 ③ 菜品未下架
  - 错误码 `DISH_OFF_SHELF`；唯一约束兜底幂等（不可改评，属既定决策）
- [x] **T5.2 赞踩数回归验证**：聚合查询已在 T2.4 实现（M2 时空表返回 0），此处验证点评写入后 `/menus`、`/menus/summary` 返回的赞踩数正确刷新（含同名跨日期合并）
- [x] **T5.3 建议**：`POST /suggestions`（员工提交）、`GET /my/suggestions`（我的列表+状态）
- [x] **T5.4 建议管理**：`GET /admin/suggestions`（按状态筛选）、`PATCH /admin/suggestions/:id`（标记 待处理/已采纳/已拒绝）
- [x] **T5.5 H5**：菜品卡片赞/踩入口（仅当日餐次显示）；我的建议页（`/my/suggestions`）
- [x] **T5.6 admin**：建议管理页（`/suggestions`）

**验收**：未就餐/未核销不可点评；下架菜品点评被拒；跨日点评被拒；同名菜品赞踩跨日期合并统计正确；建议状态流转闭环。

---

## M6 管理端统计 + 超管账号管理/看板

> 前置 M5：看板"菜品赞踩排行"依赖点评数据；统计依赖 M3 报名与 M4 核销数据。

### 统计（食堂管理员）

- [x] **T6.1** `GET /admin/stats/signups?date=`：按餐次报名人数（最新行 status ∈ signed/verified）
- [x] **T6.2** `GET /admin/stats/departments?date=`：按部门报名分布
- [x] **T6.3** `GET /admin/stats/daily?from=&to=`：实际就餐（verified + walk_in）vs 报名差异
- [x] **T6.4 admin 数据统计页**（`/statistics`：日期筛选 + 差异表 + ECharts）

### 超管

- [x] **T6.5 创建食堂管理员** `POST /super/canteen-admins`（默认密码 123456 + 强制改密）
- [x] **T6.6 用户列表** `GET /super/users?role=&keyword=&page=`（员工与食堂管理员，含状态）
- [x] **T6.7 禁用/启用** `PATCH /super/users/:id/status`（拒绝：目标为超管；拒绝：禁用超管自身）
- [x] **T6.8 重置密码** `POST /super/users/:id/reset-password`（→ 123456 + `must_change_password=true`；拒绝：目标为超管）
- [x] **T6.9 违约名单** `GET /super/violations?from=&to=`（按用户聚合推导违约 + walk_in）
- [x] **T6.10 看板四接口**：`/super/dashboard/trend|dishes|waste-rate|departments`（均支持 from/to，缺省最近 30 天；菜品按菜名合并）
  - waste-rate 口径 = 推导违约数 / 有效报名数（对齐 sheji.md §5.3 统计口径）
- [x] **T6.11 admin 超管页面**：账号管理（`/super/users`）、违约查看（`/super/violations`）、数据看板（`/super/dashboard`，ECharts）

**验收**：被禁用账号登录被拒且在线 token 被中间件 401；重置密码后旧 token 被强制改密拦截（M1 机制回归测试）；统计口径与手工 SQL 核对一致。

---

## M7 部署上线

- [x] **T7.1 Supabase 准备**：确认 eu-west-1 项目、Session Pooler（:5432）连通性；**部署前重置数据库密码**（文档 7.3 明文密码不入库）
- [x] **T7.2 迁移执行**：golang-migrate 通过 `DATABASE_URL` 执行（含超管 seed）。**时机约定：Render 免费档部署不跑迁移，须在每次 push 触发自动部署前手动/CI 执行 `migrate up`**，避免"新表未建、服务已上线"窗口；新增迁移文件时同步更新此操作习惯
- [x] **T7.3 render.yaml**：frankfurt / free / docker / `healthCheckPath: /api/v1/healthz`；`DATABASE_URL` 手动录入（sync: false）、`JWT_SECRET` 自动生成、`TZ=Asia/Shanghai`
- [x] **T7.4 绑定 GitHub 仓库**，push 自动构建；确认 Go 监听 `$PORT`
- [x] **T7.5 静态路由验证**：`/api/*` → API；`/admin*` → admin SPA；其余 → h5 SPA（含 SPA fallback）
- [x] **T7.6 上线冒烟清单**
  - admin 首登强制改密；创建食堂管理员；导入员工
  - 发布 15 天菜谱；游客查看；员工报名/取消
  - 现场核销 / 未报名登记 / 违约统计
  - 点评、建议、统计、看板全链路
  - 时区行为验证（00:00 报名截止、核销截止、点评窗口均按 Asia/Shanghai）
- [x] **T7.7 免费档行为确认**：15 分钟休眠冷启动 10~30s 预期内；不配置保活 cron

**验收**：生产环境全链路冒烟通过；数据库凭据只存在于 Render 环境变量。

---

## 附：贯穿性任务（每个里程碑内持续）

- [x] **A1 单元测试**：时间规则引擎全覆盖；核销/登记分支矩阵；违约推导口径
- [x] **A2 集成测试**：报名并发（FOR UPDATE）、核销幂等、点评校验链
- [x] **A3 错误码登记**：每新增一个错误码同步更新 api-client 与前端提示
- [x] **A4 安全与非功能自查**（对齐 sheji.md §8 全部 8 项）：SQL 参数化（sqlc）、日志无明文密码、越权路径（角色中间件覆盖所有 /admin、/super 路由）、隐私自查（无手机号字段、无手机号展示、员工列表仅工号/姓名/部门）、输入校验（后端 validator 标签 + 前端校验双端生效，日期统一 `YYYY-MM-DD`）
