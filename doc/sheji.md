# 工厂食堂报餐系统设计文档

> 依据 `doc/xuqiu.md`（已冻结版）编制。服务端 Go，数据库 Supabase PostgreSQL（eu-west-1），前端 Vue 3，部署 Render（Frankfurt，与数据库就近）+ Supabase（eu-west-1）。

## 1. 总体架构

```
 微信群分享链接
      │
      ▼
┌───────────────────────────────────────────┐
│      Render Web Service（Frankfurt）        │
│  Go 单服务（Docker 容器）                   │
│  ├─ /api/v1/*        REST API (Gin)       │
│  └─ /*               前端静态文件 (embed)   │
│       ├─ apps/h5     员工端 (Vant)         │
│       └─ apps/admin  管理端 (Element Plus) │
└──────────────────┬────────────────────────┘
                   │ pgx（Supabase Session Pooler :5432, TLS）
                   ▼
┌───────────────────────────────────────────┐
│  Supabase PostgreSQL（eu-west-1，仅用 DB） │
│  Auth/Storage/Realtime 均不使用            │
└───────────────────────────────────────────┘
```

要点：
- **单服务部署**：前端构建产物经 `go:embed` 打进 Go 二进制，一个域名、无 CORS 问题、Render 只跑一个进程。
- **认证自研**：工号 + bcrypt 密码 + JWT，与 Supabase Auth（邮箱/手机号体系）不匹配，故不用。
- **无定时任务**：违约、菜谱窗口等全部为查询时实时推导（懒计算），规避 Render 免费档无常驻 worker 的问题。
- **时区**：所有业务时间规则（当日 00:00 截止、核销 09:00/13:30/19:00、点评 23:59:59）以 `Asia/Shanghai` 为基准；服务容器 `TZ=Asia/Shanghai`；数据库字段用 `timestamptz`，业务日期用 `date`。

## 2. 技术栈

| 层 | 技术 | 版本基线 |
|---|---|---|
| 语言 | Go | 1.22+ |
| Web 框架 | Gin | v1.10 |
| DB 访问 | sqlc + pgx/v5 | sqlc 生成类型安全代码 |
| 迁移 | golang-migrate | SQL 文件版本化 |
| 认证 | golang-jwt/v5 + golang.org/x/crypto/bcrypt | HS256 |
| 校验 | go-playground/validator | v10 |
| 日志 | log/slog | 结构化 JSON |
| 员工端 | Vue 3 + TypeScript + Vite + Vant 4 | 移动 H5，PC 限宽居中 |
| 管理端 | Vue 3 + TypeScript + Vite + Element Plus | PC 左侧菜单 |
| 仓库 | pnpm monorepo | 单仓库单服务 |
| 构建 | Dockerfile 多阶段（node 构建前端 → go 构建内嵌 → 运行） | |

## 3. 仓库结构（monorepo）

```
canteen-system/
├── apps/
│   ├── h5/                    # 员工端（Vant）
│   │   └── src/
│   │       ├── views/         # Menus / History / Login / MyRecords / MySuggestions / ChangePwd
│   │       ├── components/    # DateTabs / MealTabs / DishCard / SignupButton
│   │       ├── stores/        # pinia: auth
│   │       └── router/
│   └── admin/                 # 管理端（Element Plus）
│       └── src/
│           ├── layouts/       # AdminLayout（左侧菜单 + 顶栏）
│           ├── views/         # Verify / MenuPublish / MenuItemShelf / Employees /
│           │                  # VerifyRules / Statistics / Suggestions / Password（通用改密） /
│           │                  # super: CanteenAdmins / Violations / Dashboard
│           └── router/        # 按角色守卫
├── packages/
│   └── api-client/            # 共享 TS 类型 + fetch 封装（两端复用）
├── server/
│   ├── go.mod / go.sum        # Go 依赖清单（Dockerfile 单独 COPY 以利用构建缓存）
│   ├── cmd/api/main.go
│   ├── internal/
│   │   ├── config/            # 环境变量加载
│   │   ├── auth/              # 登录、JWT、must_change_password 拦截、角色中间件
│   │   ├── user/              # 员工/食堂管理员/超管账号管理
│   │   ├── menu/              # 菜谱发布、下架恢复、查询窗口
│   │   ├── signup/            # 报名/修改/取消
│   │   ├── verify/            # 核销、未报名登记、违约推导
│   │   ├── review/            # 点评
│   │   ├── suggestion/        # 建议
│   │   ├── stat/              # 统计与看板
│   │   └── db/                # sqlc 生成代码 + 查询 SQL
│   ├── migrations/            # golang-migrate SQL（含超管 seed）
│   └── static/                # go:embed 前端产物挂载点（源码内放 .gitkeep 占位，
│                              #  构建时由 Dockerfile 注入 h5/admin 产物）
├── package.json               # 根包：声明 packageManager 字段固定 pnpm 版本（corepack 依赖）
├── pnpm-lock.yaml             # 锁文件（Dockerfile COPY 依赖，--frozen-lockfile 安装）
├── Dockerfile                 # 多阶段构建（位于仓库根，构建上下文 = 仓库根）
├── render.yaml
└── pnpm-workspace.yaml
```

## 4. 数据库设计

### 4.1 枚举与公共约定

- 角色：`user_role = ('employee','canteen_admin','super_admin')`
- 餐次：`meal_type = ('breakfast','lunch','dinner')`
- 所有表带 `created_at timestamptz not null default now()`；更新型表加 `updated_at`（例外：`meal_signups` 不加——其状态变更时间已由 `cancelled_at` 与 `verifications.verified_at` 完整记录）。
- 工号 `emp_no` 全局唯一，三类账号共用 `users` 表。

### 4.2 DDL（核心部分）

```sql
-- ========== 枚举类型 ==========
create type user_role as enum ('employee','canteen_admin','super_admin');
create type meal_type as enum ('breakfast','lunch','dinner');
comment on type user_role is '用户角色：employee=普通员工 / canteen_admin=食堂管理员 / super_admin=超级管理员';
comment on type meal_type is '餐次：breakfast=早餐 / lunch=午餐 / dinner=晚餐';

-- ========== 用户表 ==========
create table users (
  id                   bigserial primary key,
  emp_no               text not null unique,
  name                 text not null,
  department           text not null default '',
  password_hash        text not null,
  role                 user_role not null default 'employee',
  status               text not null default 'active'
                       check (status in ('active','disabled')),
  must_change_password boolean not null default true,
  created_at           timestamptz not null default now(),
  updated_at           timestamptz not null default now()
);
comment on table  users is '用户表：普通员工/食堂管理员/超级管理员共用';
comment on column users.id is '主键，自增';
comment on column users.emp_no is '登录名（工号）；超级管理员固定为 admin';
comment on column users.name is '姓名';
comment on column users.department is '部门；超级管理员留空';
comment on column users.password_hash is '密码哈希（bcrypt）';
comment on column users.role is '角色：employee / canteen_admin / super_admin';
comment on column users.status is '账号状态：active=正常 / disabled=禁用（禁用后无法登录）';
comment on column users.must_change_password is '是否强制修改密码：首登或被重置后为 true，改密成功后置 false';
comment on column users.created_at is '创建时间';
comment on column users.updated_at is '最后更新时间';
create index idx_users_role_status on users(role, status);
create index idx_users_department on users(department);

-- ========== 菜谱表（发布即落库，无草稿态） ==========
create table menu_items (
  id          bigserial primary key,
  menu_date   date not null,
  meal        meal_type not null,
  dish_name   text not null,
  taste_tags  text[] not null default '{}',
  status      text not null default 'normal'
              check (status in ('normal','off_shelf')),
  created_at  timestamptz not null default now(),
  updated_at  timestamptz not null default now(),
  unique (menu_date, meal, dish_name)
);
comment on table  menu_items is '菜谱表：发布即落库，无草稿态；已发布不可编辑，只能下架/恢复（下架/恢复时更新 updated_at）';
comment on column menu_items.id is '主键，自增';
comment on column menu_items.menu_date is '菜谱日期（发布范围：今日往后最多 15 天，含今日）';
comment on column menu_items.meal is '餐次：breakfast / lunch / dinner';
comment on column menu_items.dish_name is '菜品名称；同名菜品跨日期按菜名合并统计赞踩数';
comment on column menu_items.taste_tags is '口味标签数组，如 {微辣, 清淡}';
comment on column menu_items.status is '菜品状态：normal=正常 / off_shelf=已下架（下架仅展示标识，不影响按餐次报名）';
comment on column menu_items.created_at is '发布时间';
comment on constraint menu_items_menu_date_meal_dish_name_key on menu_items
  is '同一日期同一餐次内菜品名唯一（复制/发布防重）';
create index idx_menu_date_meal on menu_items(menu_date, meal);

-- ========== 核销规则表（仅存覆盖值） ==========
create table verify_rules (
  meal             meal_type primary key,
  verify_deadline  time not null,
  updated_at       timestamptz not null default now()
);
comment on table  verify_rules is '核销规则表：仅存管理员覆盖值，未配置走代码默认；覆盖保存时更新 updated_at';
comment on column verify_rules.meal is '餐次（主键）';
comment on column verify_rules.verify_deadline is '核销截止时间（覆盖值）；默认值：早餐 09:00 / 午餐 13:30 / 晚餐 19:00（Asia/Shanghai）';

-- ========== 报餐记录表 ==========
create table meal_signups (
  id            bigserial primary key,
  user_id       bigint not null references users(id),
  menu_date     date not null,
  meal          meal_type not null,
  status        text not null default 'signed'
                check (status in ('signed','cancelled','verified')),
  signed_at     timestamptz not null default now(),
  cancelled_at  timestamptz
);
comment on table  meal_signups is '报餐记录表：修改=旧行置 cancelled+新行 signed，同一餐次允许多条历史行，有效状态取 signed_at 最新一行；违约为推导态（signed 且已过核销截止），不落库';
comment on column meal_signups.id is '主键，自增';
comment on column meal_signups.user_id is '报餐员工的用户 ID';
comment on column meal_signups.menu_date is '用餐日期（报名/取消须在用餐当日 00:00 前）';
comment on column meal_signups.meal is '餐次';
comment on column meal_signups.status is '报餐状态：signed=已报名 / cancelled=已取消 / verified=已核销';
comment on column meal_signups.signed_at is '本行报餐时间（修改即产生新行）';
comment on column meal_signups.cancelled_at is '取消时间（被修改或主动取消时写入）';
create index idx_signup_user_date_meal on meal_signups(user_id, menu_date, meal, signed_at desc);
create index idx_signup_agg on meal_signups(menu_date, meal, status);
-- 注意：不设 (user_id, menu_date, meal) 唯一约束——历史行需保留

-- ========== 核销记录表 ==========
create table verifications (
  id           bigserial primary key,
  signup_id    bigint not null unique references meal_signups(id),
  verified_by  bigint not null references users(id),
  verified_at  timestamptz not null default now()
);
comment on table  verifications is '核销记录表：员工报工号、食堂管理员确认核销';
comment on column verifications.id is '主键，自增';
comment on column verifications.signup_id is '对应报餐记录 ID（唯一约束防止重复核销）';
comment on column verifications.verified_by is '核销人（食堂管理员用户 ID）';
comment on column verifications.verified_at is '核销时间';

-- ========== 未报名就餐记录表 ==========
create table walk_in_records (
  id            bigserial primary key,
  user_id       bigint not null references users(id),
  menu_date     date not null,
  meal          meal_type not null,
  registered_by bigint not null references users(id),
  registered_at timestamptz not null default now(),
  unique (user_id, menu_date, meal)
);
comment on table  walk_in_records is '未报名就餐记录表：未报名但到现场就餐的登记，计入违约；与核销互斥（有报餐走核销，无报餐走登记）';
comment on column walk_in_records.id is '主键，自增';
comment on column walk_in_records.user_id is '就餐员工用户 ID';
comment on column walk_in_records.menu_date is '用餐日期';
comment on column walk_in_records.meal is '餐次';
comment on column walk_in_records.registered_by is '登记人（食堂管理员用户 ID）';
comment on column walk_in_records.registered_at is '登记时间';
comment on constraint walk_in_records_user_id_menu_date_meal_key on walk_in_records
  is '同一员工同一餐次仅一条未报名就餐记录（幂等兜底）';

-- ========== 菜品点评表 ==========
create table dish_reviews (
  id          bigserial primary key,
  user_id     bigint not null references users(id),
  menu_date   date not null,
  meal        meal_type not null,
  dish_name   text not null,
  rating      smallint not null check (rating in (1,-1)),
  created_at  timestamptz not null default now(),
  unique (user_id, menu_date, meal, dish_name)
);
comment on table  dish_reviews is '菜品点评表：仅当餐已核销或已登记未报名就餐、菜品未下架、当日 23:59:59 前可写';
comment on column dish_reviews.id is '主键，自增';
comment on column dish_reviews.user_id is '点评员工用户 ID';
comment on column dish_reviews.menu_date is '点评针对的用餐日期（须为当日）';
comment on column dish_reviews.meal is '餐次';
comment on column dish_reviews.dish_name is '菜品名称（同名合并统计的聚合键）';
comment on column dish_reviews.rating is '评价类型：1=赞 / -1=踩';
comment on column dish_reviews.created_at is '评价时间（校验当日 23:59:59 前有效）';
comment on constraint dish_reviews_user_id_menu_date_meal_dish_name_key on dish_reviews
  is '一人一餐一菜仅一次评价';
create index idx_review_agg on dish_reviews(dish_name, rating);

-- ========== 建议表 ==========
create table suggestions (
  id          bigserial primary key,
  user_id     bigint not null references users(id),
  content     text not null,
  status      text not null default 'pending'
              check (status in ('pending','adopted','rejected')),
  created_at  timestamptz not null default now()
);
comment on table  suggestions is '菜品建议表：员工提交，食堂管理员标记处理状态';
comment on column suggestions.id is '主键，自增';
comment on column suggestions.user_id is '提建议员工用户 ID';
comment on column suggestions.content is '建议内容';
comment on column suggestions.status is '处理状态：pending=待处理 / adopted=已采纳 / rejected=已拒绝';
comment on column suggestions.created_at is '提交时间';
```

### 4.3 种子数据（migration 内）

```sql
-- 超级管理员（系统唯一，不可被禁用）
-- 注意：<bcrypt(admin123)> 为占位符，迁移执行前须替换为真实哈希，
-- 生成方式示例：htpasswd -bnBC 10 "" admin123 | tr -d ':\n'
insert into users (emp_no, name, department, password_hash, role)
values ('admin', 'admin', '', '<bcrypt(admin123)>', 'super_admin');
```

### 4.4 状态模型与违约推导

**报餐记录**存储三态 `signed / cancelled / verified`；**违约是推导态**，不落库：

```
有效报名（最新一行的 status）
├─ signed   + now() > 核销截止时间  → 展示为「违约」（no_show）
├─ signed   + now() ≤ 核销截止时间  → 展示为「已报名」
├─ verified                              → 「已核销」
└─ cancelled                             → 「已取消」
违约次数 = 推导违约的报餐数 + walk_in_records 数
```

核销截止时间 = `verify_rules` 中该餐次的覆盖值，缺省用代码常量：
`breakfast 09:00 / lunch 13:30 / dinner 19:00`（Asia/Shanghai）。

## 5. 后端设计

### 5.1 时间规则引擎（单点实现）

所有时间边界集中在 `internal/verify/timewindow.go`（或独立 `internal/timewindow`），杜绝散落判断：

| 规则 | 判定 |
|---|---|
| 可报名/修改/取消 | `now < menu_date 当日 00:00`（Asia/Shanghai） |
| 菜谱可发布范围 | `today ≤ menu_date ≤ today+14`（含今日共 15 天） |
| 菜谱可见范围（未来） | 同上 |
| 历史菜谱可见范围 | `menu_date ≥ today-30` |
| 核销截止 | `verify_rules` 覆盖值 \|\| 默认常量；核销操作须在截止前完成，过期拒绝并视为违约 |
| 核销/登记目标日期 | 核销查询、执行核销、未报名登记的 `menu_date` 必须为当日（防提前核销未来餐次） |
| 点评窗口 | `当日 00:00 ≤ now ≤ 当日 23:59:59` |

注：`verify_rules` 调整即时生效于违约推导（含当日餐次）。由于违约为懒计算不落库，规则变更会重算历史判定——管理员应仅在餐次核销截止前调整规则；如需防滥用，接口可拒绝修改"截止时间已过"的餐次规则。

### 5.2 认证与权限

- **登录**：`POST /api/v1/auth/login` → 校验 bcrypt → 签发 JWT（claims：`uid`、`role`、`mcp`(must_change_password)）。
- **记住登录**：`remember=true` 时 token 有效期 30 天（前端存 localStorage）；否则 12 小时（存 sessionStorage）。续期由前端静默重登录或下次登录完成，不做 refresh token（系统规模不需要）。
- **首登强制改密**：JWT 内 `mcp=true` 时，中间件除 `PUT /my/password`、`GET /me` 外一律返回 `403 MUST_CHANGE_PASSWORD`；改密成功后重新签发 `mcp=false` 的 token。修改密码仅此一个接口（`PUT /my/password`，旧密码+新密码），首登强制与个人中心共用。
- **账号禁用与密码重置后的 token 失效**：登录时拒绝禁用账号；已被禁用但持有未过期 token 的请求，中间件校验 `users.status`（可缓存 60s）并返回 401。同时中间件比对 DB 的 `must_change_password` 与 token 内 `mcp` claim：不一致时以 DB 为准返回 `403 MUST_CHANGE_PASSWORD`——防止密码被超管重置后，旧 token（mcp=false）继续绕过强制改密。
- **角色中间件**：`RequireRole(employee)` / `RequireRole(canteen_admin)` / `RequireRole(super_admin)`，路由级挂载。
- **防爆破**：登录接口按 `emp_no+IP` 做失败计数，5 次失败锁定 10 分钟（内存实现即可，单实例）。

### 5.3 关键业务流程

**报名/修改/取消**（事务内，`SELECT ... FOR UPDATE` 锁定该用户最新报餐行实现并发安全）：

```
报名：  校验菜单已发布（该日期+餐次存在任意 menu_items 记录即视为已发布；
        菜品全部下架也不影响报名，因报餐按餐次）
      → 校验 now < menu_date 00:00 → INSERT signed 行
修改：  同报名（旧行 UPDATE 为 cancelled + cancelled_at，新行 INSERT signed）
取消：  校验 now < menu_date 00:00 → 最新行 UPDATE 为 cancelled
```

**发布菜谱**（事务）：

```
校验所有 menu_date ∈ [today, today+14]（任一行日期越界 → 整批拒绝）
→ 单事务内逐行 INSERT menu_items，同餐同名冲突行跳过并在响应中返回
失败明细，其余行提交（与员工导入模式一致）
复制上周：返回上周一至周日的菜品模板（按 星期几+餐次 组织，不绑定目标日期）；
前端选择目标日期（本周剩余日或下周），逐日映射、修改后走正常发布接口校验发布范围
```

**现场核销 / 未报名登记**（互斥，事务内先查后写）：

```
输入工号 → 校验 menu_date 为当日（lookup/verify/walk-in 均只允许操作当日餐次，
防止提前核销未来餐次绕过违约判定）→ 查该员工当餐最新报餐记录：
├─ 最新行 signed：
│    ├─ now ≤ 核销截止 → 展示报餐信息 → 确认核销：
│    │    UPDATE meal_signups SET status='verified' + INSERT verifications
│    └─ now > 核销截止 → 该餐已判违约：核销与未报名登记均拒绝，
│         防止同一餐次重复计入违约
├─ 已 verified → 提示已核销，防止重复操作
└─ 最新行 cancelled 或无记录 → 登记「未报名就餐」：
     INSERT walk_in_records（ON CONFLICT 幂等；若已存在提示勿重复登记）

注：核销截止时间约束仅作用于「核销」分支（signed 行过期即判违约、不可再核销，
保护违约判定的有效性）；「未报名就餐」分支按需求约定不额外限制登记时点
（menu_date 须为当日，当餐时段由工作人员现场把握）。
「有效报餐」= 最新行 status ∈ (signed, verified)。
```

**点评校验链**（写入前依次判定，任一失败即拒绝）：

```
① 当日 00:00 ≤ now ≤ 23:59:59
② 该员工该餐存在 verified 报餐 或 walk_in_records 记录
③ menu_items 中该 (menu_date, meal, dish_name) 状态为 normal（未下架）
```

**统计口径**：

- 报餐人数：最新行 status ∈ (signed, verified) 按日期餐次计数。
- 实际就餐 = verified 数 + walk_in 数。
- 报餐未到率（浪费率）= (推导违约数) / (有效报名数)。
- 违约名单：按用户聚合「推导违约 + 未报名就餐」。
- 菜品排行：`dish_reviews` 按 `dish_name` 聚合 `sum(case rating when 1 then 1 else 0 end)` 赞数、踩数。

### 5.4 API 清单（/api/v1）

**公开（游客）**

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/menus?start=&end=` | 菜谱查询（未来 ≤15 天 / 历史 ≤30 天，越界自动截断）；返回范围内全部菜品（含已下架，附 status 供前端打标），并含每菜赞踩数（同名合并） |
| GET | `/menus/summary?date=` | 单日三餐聚合（单日刷新/深链场景备用；首页首屏用上方范围接口一次拉取 15 天） |
| GET | `/healthz` | 健康检查（Render 探活，无需认证；见 §8） |

**认证**

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/auth/login` | 工号+密码（remember 可选） |
| GET | `/me` | 当前用户信息 |
| PUT | `/my/password` | 修改自己的密码（**所有角色通用**：员工/食堂管理员/超管；旧密码+新密码，首登强制与个人中心共用） |

**员工端（employee）**

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/signups` | 报名/修改 `{menu_date, meal}` |
| DELETE | `/signups?menu_date=&meal=` | 取消 |
| GET | `/my/signups?from=&to=` | 用餐记录：报餐记录（含推导违约状态）与未报名就餐记录合并返回（供员工核对违约明细） |
| GET | `/my/violation-count` | 违约次数 |
| POST | `/reviews` | 点赞/踩 `{menu_date, meal, dish_name, rating}` |
| GET | `/my/suggestions` | 我的建议列表（含处理状态） |
| POST | `/suggestions` | 提交建议 |

**食堂管理员端（canteen_admin）**

| 方法 | 路径 | 说明 |
|---|---|---|
| GET/POST | `/admin/employees` | 员工列表 / 手动添加（**仅超管**；需求变更：员工账号管理归超管） |
| POST | `/admin/employees/import` | CSV 批量导入（逐行校验，返回成功/失败明细；**仅超管**） |
| PATCH | `/admin/employees/:id` | 编辑（仅姓名、部门，工号不可改；**仅超管**） |
| POST | `/admin/menus` | 发布菜谱（一次可提交多天多餐：`[{menu_date, meal, dishes: [{dish_name, taste_tags}]}]`；单事务处理：日期越界整批拒绝，同餐同名冲突行跳过并返回明细） |
| GET | `/admin/menus/copy-last-week` | 返回上周菜单模板（按星期几+餐次组织，不绑定目标日期） |
| PATCH | `/admin/menu-items/:id/shelf` | 下架 / 恢复 |
| GET/PUT | `/admin/verify-rules` | 核销规则查看 / 覆盖 |
| GET | `/admin/stats/signups?date=` | 按餐次报名人数 |
| GET | `/admin/stats/departments?date=` | 按部门报名分布 |
| GET | `/admin/stats/daily?from=&to=` | 核销数+未报名数 vs 报名数差异 |
| GET | `/admin/stats/attendance-detail?date=&dept=&keyword=&page=&page_size=` | 某天用餐明细分页：全员×三餐状态（已核销/报名未到/未报名就餐/已取消/未报名），支持部门与工号/姓名筛选——数据统计页"用餐明细"数据源 |
| GET | `/admin/verify/lookup?emp_no=&menu_date=&meal=` | 核销前查询（menu_date 须为当日，拒绝未来/历史日期） |
| POST | `/admin/verify` | 执行核销（menu_date 须为当日；须在核销截止前，过期返回 `VERIFY_CLOSED`） |
| POST | `/admin/walk-in` | 未报名就餐登记（menu_date 须为当日，拒绝未来日期） |
| GET | `/admin/suggestions` | 建议列表（可按状态筛选） |
| PATCH | `/admin/suggestions/:id` | 标记处理状态 |

**超级管理员端（super_admin）**

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/super/canteen-admins` | 创建食堂管理员（工号、姓名、部门） |
| GET | `/super/users?role=&keyword=&page=` | 用户列表（员工与食堂管理员，含账号状态；账号管理页数据源） |
| PATCH | `/super/users/:id/status` | 禁用/启用（拒绝：目标为超管；拒绝：禁用超管自身） |
| POST | `/super/users/:id/reset-password` | 重置密码 → 123456 + must_change_password=true（拒绝：目标为超管；超管自身改密走 `/my/password`） |
| GET | `/super/violations?from=&to=` | 违约员工名单及次数 |
| GET | `/super/dashboard/trend?from=&to=` | 每日就餐人数趋势（看板四个接口均支持 from/to，缺省最近 30 天） |
| GET | `/super/dashboard/dishes` | 菜品赞踩排行（按菜名合并） |
| GET | `/super/dashboard/waste-rate` | 报餐未到率 |
| GET | `/super/dashboard/departments` | 各部门就餐对比 |

**统一响应**：`{code, msg, data}`；错误码枚举（如 `MUST_CHANGE_PASSWORD`、`SIGNUP_CLOSED`、`VERIFY_CLOSED`、`DISH_OFF_SHELF`、`ALREADY_VERIFIED`）供前端精确提示。

### 5.5 Excel/CSV 导入

- 后端解析（Excel 用 `excelize`，CSV 标准库），模板列：`工号,姓名,部门`。文件编码统一 UTF-8（CSV 建议带 BOM，兼容 Excel 导出的中文）。
- 逐行校验：工号非空且不重复（文件内 + 库内）、部门可空。
- 结果响应：`{success_count, failures:[{row, reason}]}`，前端展示失败明细；成功行即时生效（默认密码 123456）。

## 6. 前端设计

### 6.1 员工端 H5（apps/h5，Vant 4）

**路由**：

| 路径 | 页面 | 说明 |
|---|---|---|
| `/menus` | 首页菜谱 | 默认页；日期横滑 Tab（15 天）+ 早/午/晚 Tab |
| `/history` | 历史菜谱 | 日期范围选择（≤30 天）+ 快捷"上周" |
| `/login` | 登录 | 工号+密码+记住登录 |
| `/my/records` | 用餐记录 | 报餐/核销/违约列表 + 违约次数卡片 |
| `/my/suggestions` | 我的建议 | 列表+状态 |
| `/my` | 我的 | 个人中心（点评入口在菜谱卡片上） |
| `/my/password` | 修改密码 | 首登强制跳转此页 |

**Tabbar**：`菜谱 | 记录 | 我的`（Vant Tabbar）。

**菜谱页结构**：

```
┌──────────────────────────┐
│ 日期 Tab（横向滑动 15 天）│
│ [早] [午] [晚]           │
├──────────────────────────┤
│ DishCard × N             │
│  菜名  口味标签           │
│  赞 12 / 踩 3            │
│  [已下架] （下架时置灰）  │
│                          │
│ [报名 / 已报名 / 已锁定]  │ ← 按餐次维度的报名条
└──────────────────────────┘
```

- 报名按钮按**餐次**展示（需求：报餐不选菜品），菜品卡片仅展示信息。
- 点评入口（赞/踩）仅在**当日**餐次的菜品卡片上显示（点评限当日 23:59:59 前，未来与历史日期不显示）。
- 已过当日 00:00 的餐次按钮置灰为「已锁定」。
- 首屏获取：`GET /menus?start=<今日>&end=<今日+14>`（一次拉取 15 天，渲染日期 Tab 与当日菜品）；已登录时再调 `GET /my/signups?from=&to=` 合并报名状态。

**PC 适配**：根容器 `max-width: 480px; margin: 0 auto`（CSS 一行，两端生效）。

### 6.2 管理端（apps/admin，Element Plus）

**布局**：`AdminLayout` = 左侧菜单（可折叠）+ 顶栏（面包屑 + 用户名/退出）。菜单按角色渲染。admin 应用以 `/admin` 为路由 base（Vite `base: '/admin/'`），下表路径的实际 URL 前缀均为 `/admin`。

**路由与菜单**（食堂管理员，按使用频率排序）：

| 菜单 | 路径 | 页面要点 |
|---|---|---|
| 现场核销 | `/verify` | 工号输入框（自动聚焦）→ 显示当餐记录 → [核销] / [登记未报名就餐] |
| 菜谱发布 | `/menus/publish` | 周视图选择 + 菜品编辑（添加行：菜名+口味标签）+ [复制上周] |
| 菜品下架 | `/menus/items` | 按日期餐次列出菜品，[下架]/[恢复] 开关 |
| 核销规则 | `/verify-rules` | 三餐核销截止时间（默认值占位，可覆盖保存） |
| 数据统计 | `/statistics` | 日期筛选：餐次报名数 / 部门分布 / 差异表 + ECharts 图 |
| 建议管理 | `/suggestions` | 列表 + 状态标记（待处理/已采纳/已拒绝） |
| 修改密码 | `/password` | 旧密码+新密码；所有角色（含超管）个人中心入口 |

**超级管理员菜单**（不包含食堂管理员的核销/菜谱/统计等页面）：

| 菜单 | 路径 | 页面要点 |
|---|---|---|
| 员工管理 | `/employees` | 表格 + 手动添加 + CSV 导入（结果明细弹窗）+ 编辑（姓名/部门） |
| 账号管理 | `/super/users` | 食堂管理员创建/列表、员工与食堂管理员禁用/启用、重置密码 |
| 违约查看 | `/super/violations` | 违约名单表格（姓名、部门、违约次数、明细） |
| 数据看板 | `/super/dashboard` | 趋势折线、菜品赞踩排行、未到率、部门对比（ECharts） |
| 修改密码 | `/password` | 所有角色通用 |

**登录落点与强制改密**：登录响应携带 `must_change_password`——为 true 时前端**立即跳改密页**（不依赖后续 API 的 403 拦截）；角色不匹配的页面访问由守卫重定向回本角色首页（食堂管理员→`/verify`，超管→`/super/users`）。员工账号在管理端 URL 登录时整页跳转员工端。

### 6.3 共享 api-client（packages/api-client）

- TypeScript 接口定义（与后端 DTO 一一对应）。
- `request()` 封装：自动带 JWT、401 跳登录、`MUST_CHANGE_PASSWORD` 跳改密、统一错误 toast。

## 7. 部署设计

### 7.1 Dockerfile（多阶段）

```dockerfile
# 1. 构建前端（构建上下文 = 仓库根，Dockerfile 位于根目录）
FROM node:20-alpine AS frontend
WORKDIR /build
RUN corepack enable            # 根 package.json 须声明 packageManager 字段以固定 pnpm 版本
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./
COPY apps apps
COPY packages packages
RUN pnpm install --frozen-lockfile \
 && pnpm --filter @canteen/h5 build \
 && pnpm --filter @canteen/admin build

# 2. 构建 Go（embed 用 all: 前缀；源码内 static 放 .gitkeep 保证空目录可嵌入）
FROM golang:1.22-alpine AS server
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ .
COPY --from=frontend /build/apps/h5/dist     ./static/h5
COPY --from=frontend /build/apps/admin/dist  ./static/admin
RUN CGO_ENABLED=0 go build -o /app ./cmd/api

# 3. 运行
FROM alpine:3.20
RUN apk add --no-cache tzdata && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
COPY --from=server /app /app
ENV TZ=Asia/Shanghai
EXPOSE 8080
ENTRYPOINT ["/app"]
```

### 7.2 Render 配置（render.yaml）

```yaml
services:
  - type: web
    name: canteen-system
    runtime: docker          # Dockerfile 位于仓库根，构建上下文即仓库根
    plan: free
    region: frankfurt        # 与 Supabase eu-west-1（爱尔兰）就近，降低 DB 往返延迟
    healthCheckPath: /api/v1/healthz
    envVars:
      - key: DATABASE_URL    # Supabase Session Pooler 连接串（见 7.3）
        sync: false
      - key: JWT_SECRET
        generateValue: true
      - key: TZ
        value: Asia/Shanghai
```

- 绑定 GitHub 仓库，push 自动构建部署。
- **端口监听**：Render 会向 Docker 服务注入 `PORT` 环境变量，Go 服务须监听 `0.0.0.0:$PORT`（本地缺省 8080）；Dockerfile 中 `EXPOSE 8080` 仅为本地开发约定。
- 静态路由策略（Go 内实现）：`/api/*` 优先匹配走 API；`/admin*` 与 `/admin/assets/*` 返回 admin SPA；其余返回 h5 SPA（均带 SPA fallback 到 index.html）。

### 7.3 Supabase 配置

- 项目实际区域为 **eu-west-1（爱尔兰）**，Session Pooler 域名为 `aws-0-eu-west-1.pooler.supabase.com`。
- **连接参数（已确定）**：

| 项 | 值 |
|---|---|
| host | `aws-0-eu-west-1.pooler.supabase.com` |
| port | `5432`（Session Pooler） |
| database | `postgres` |
| user | `postgres.yikizzibtsokolgnnkfi` |
| password | `MyDb2026Secure!789` |

- 完整 `DATABASE_URL`：

  ```
  postgresql://postgres.yikizzibtsokolgnnkfi:MyDb2026Secure!789@aws-0-eu-west-1.pooler.supabase.com:5432/postgres?sslmode=require
  ```

- Session 模式兼容 pgx 的 prepared statements；pooler 域名走 IPv4，Render 容器与本地开发机均可直连；`db.<项目ref>.supabase.co:5432` 直连串为 IPv6-only（Render 容器无 IPv6 出口），不使用。
- 不使用 **Transaction Pooler（端口 6543）**：transaction 模式与 pgx 默认语句缓存不兼容；若将来切换，必须配置 `default_query_exec_mode=simple_protocol`。
- 迁移（`golang-migrate up`）通过同一连接串在本地或 CI 执行。
- RLS 不启用（无 Supabase Auth，访问控制全部在 Go 层）。
- **区域匹配**：Supabase 项目在 eu-west-1（爱尔兰），Render 服务选 **Frankfurt**（地理最近）以降低往返延迟；如改回 Singapore 亦可运行，但每次 DB 查询约多 200ms。
- **安全提示**：以上密码已写入本文档。若仓库会推送到远端 Git，建议部署前在 Supabase 控制台重置数据库密码、只在 Render 环境变量中录入新值，避免凭据随代码库泄漏。

### 7.4 环境变量清单

| 变量 | 说明 |
|---|---|
| `DATABASE_URL` | Supabase Session Pooler 连接串（运行时与迁移共用） |
| `JWT_SECRET` | 签名密钥（Render 自动生成） |
| `TZ` | Asia/Shanghai |

### 7.5 免费档行为（已知会，不做保活）

- Render free：15 分钟无流量休眠，下次访问冷启动约 10~30 秒（首次打开链接稍慢，属预期）。
- Supabase free：一周无请求会暂停项目，需手动恢复（系统有人用即不会触发）。
- 按约定**不配置保活 cron**。

## 8. 非功能设计

| 项 | 方案 |
|---|---|
| 密码安全 | bcrypt cost=10；默认密码 123456/admin123 仅作初始值，首登强制修改 |
| 隐私 | 不采集、不展示手机号；员工列表仅工号/姓名/部门 |
| 输入校验 | validator 标签 + 前端双端校验；日期统一 `YYYY-MM-DD` |
| 并发安全 | 报名/核销走行级锁事务；核销、未报名登记、点评均有唯一约束兜底幂等 |
| SQL 注入 | sqlc 参数化查询，无拼接 |
| 日志 | slog JSON，记录 request_id、uid、路径、耗时；不打密码明文 |
| 健康检查 | `GET /api/v1/healthz`（Render 探活） |
| 性能 | 数据量小（数百人 × 15 天），单实例 + 索引足够；统计查询均走聚合索引 |

## 9. 里程碑建议

1. M1 数据库迁移 + 认证（登录/首登改密/JWT/角色中间件）
2. M2 菜谱发布/查询 + 员工端 H5 首页
3. M3 报名/取消/用餐记录
4. M4 核销 + 未报名登记 + 违约推导
5. M5 点评 + 建议
6. M6 管理端统计 + 超管账号管理/看板
7. M7 部署上线（Render + Supabase）
