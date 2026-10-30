-- 超级管理员种子数据（系统唯一，不可被禁用）
-- bcrypt 哈希为 admin123（bcrypt cost=10），由 cmd/genhash 工具生成：
--   go run ./cmd/genhash admin123
-- 幂等：仅在不存在 emp_no='admin' 的超管时插入
insert into users (emp_no, name, department, password_hash, role, must_change_password)
select 'admin', 'admin', '',
       '$2a$10$UnxVvSKwj2V8n04TdiqnW.wuXA0qihR2HaHF6ItE4IWlK6h97s/46',
       'super_admin', true
where not exists (
  select 1 from users where emp_no = 'admin' and role = 'super_admin'
);
