-- 演示模式（DEMO_MODE 默认 true）：超管 admin 密码固定为 123123，且不允许在系统中修改。
-- bcrypt 哈希由 cmd/genhash 生成：go run ./cmd/genhash 123123
-- 幂等：重复执行无副作用（始终固定为演示密码）
update users
set password_hash = '$2a$10$uxS3mNmRZp3GruqE7xs8d.t6GAhQdn0YYRDV7.lVhbN.3ao4y4ppG',
    must_change_password = false
where emp_no = 'admin' and role = 'super_admin';
