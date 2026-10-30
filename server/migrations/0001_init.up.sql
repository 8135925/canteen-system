-- ========== 工厂食堂报餐系统 初始建表（对齐 doc/sheji.md §4.2） ==========

-- 枚举类型
create type user_role as enum ('employee','canteen_admin','super_admin');
create type meal_type as enum ('breakfast','lunch','dinner');
comment on type user_role is '用户角色：employee=普通员工 / canteen_admin=食堂管理员 / super_admin=超级管理员';
comment on type meal_type is '餐次：breakfast=早餐 / lunch=午餐 / dinner=晚餐';

-- 用户表：普通员工/食堂管理员/超级管理员共用
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
comment on column users.emp_no is '登录名（工号）；超级管理员固定为 admin';
comment on column users.role is '角色：employee / canteen_admin / super_admin';
comment on column users.status is '账号状态：active=正常 / disabled=禁用（禁用后无法登录）';
comment on column users.must_change_password is '是否强制修改密码：首登或被重置后为 true，改密成功后置 false';
create index idx_users_role_status on users(role, status);
create index idx_users_department on users(department);

-- 菜谱表：发布即落库，无草稿态；已发布不可编辑，只能下架/恢复
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
comment on column menu_items.menu_date is '菜谱日期（发布范围：今日往后最多 15 天，含今日）';
comment on column menu_items.dish_name is '菜品名称；同名菜品跨日期按菜名合并统计赞踩数';
comment on column menu_items.status is '菜品状态：normal=正常 / off_shelf=已下架';
create index idx_menu_date_meal on menu_items(menu_date, meal);

-- 核销规则表：仅存管理员覆盖值，未配置走代码默认
create table verify_rules (
  meal             meal_type primary key,
  verify_deadline  time not null,
  updated_at       timestamptz not null default now()
);
comment on table verify_rules is '核销规则表：仅存管理员覆盖值，未配置走代码默认；覆盖保存时更新 updated_at';

-- 报餐记录表：修改=旧行置 cancelled+新行 signed，有效状态取 signed_at 最新一行；违约为推导态不落库
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
create index idx_signup_user_date_meal on meal_signups(user_id, menu_date, meal, signed_at desc);
create index idx_signup_agg on meal_signups(menu_date, meal, status);

-- 核销记录表
create table verifications (
  id           bigserial primary key,
  signup_id    bigint not null unique references meal_signups(id),
  verified_by  bigint not null references users(id),
  verified_at  timestamptz not null default now()
);
comment on table verifications is '核销记录表：员工报工号、食堂管理员确认核销；signup_id 唯一约束防止重复核销';

-- 未报名就餐记录表：计入违约；与核销互斥
create table walk_in_records (
  id            bigserial primary key,
  user_id       bigint not null references users(id),
  menu_date     date not null,
  meal          meal_type not null,
  registered_by bigint not null references users(id),
  registered_at timestamptz not null default now(),
  unique (user_id, menu_date, meal)
);
comment on table  walk_in_records is '未报名就餐记录表：未报名但到现场就餐的登记，计入违约；与核销互斥';
comment on constraint walk_in_records_user_id_menu_date_meal_key on walk_in_records
  is '同一员工同一餐次仅一条未报名就餐记录（幂等兜底）';

-- 菜品点评表
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
create index idx_review_agg on dish_reviews(dish_name, rating);

-- 菜品建议表
create table suggestions (
  id          bigserial primary key,
  user_id     bigint not null references users(id),
  content     text not null,
  status      text not null default 'pending'
              check (status in ('pending','adopted','rejected')),
  created_at  timestamptz not null default now()
);
comment on table suggestions is '菜品建议表：员工提交，食堂管理员标记处理状态';
