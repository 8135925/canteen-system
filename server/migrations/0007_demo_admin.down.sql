-- down：恢复初始种子密码 admin123（须手动重新执行首登改密流程）
update users
set password_hash = '$2a$10$UnxVvSKwj2V8n04TdiqnW.wuXA0qihR2HaHF6ItE4IWlK6h97s/46',
    must_change_password = true
where emp_no = 'admin' and role = 'super_admin';
