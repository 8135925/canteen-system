# 端到端冒烟测试：对真实数据库执行全链路验证（迁移 → 登录 → 菜谱 → 报名 → 核销 → 统计）
$ErrorActionPreference = "Stop"
$Base = "http://localhost:8080/api/v1"

function Post($path, $body, $token) {
    $h = @{ "Content-Type" = "application/json" }
    if ($token) { $h["Authorization"] = "Bearer $token" }
    return Invoke-RestMethod -Method Post -Uri "$Base$path" -Headers $h -Body ($body | ConvertTo-Json -Depth 8)
}
function GetReq($path, $token) {
    $h = @{}
    if ($token) { $h["Authorization"] = "Bearer $token" }
    return Invoke-RestMethod -Method Get -Uri "$Base$path" -Headers $h
}
function Put($path, $body, $token) {
    $h = @{ "Content-Type" = "application/json"; "Authorization" = "Bearer $token" }
    return Invoke-RestMethod -Method Put -Uri "$Base$path" -Headers $h -Body ($body | ConvertTo-Json -Depth 8)
}

$pass = 0; $fail = 0
function Assert($name, $cond) {
    if ($cond) { Write-Host "PASS  $name"; $script:pass++ }
    else { Write-Host "FAIL  $name"; $script:fail++ }
}

# 1. 健康检查
$health = GetReq "/healthz" $null
Assert "健康检查" ($health.data.status -eq "up")

# 2. 超管登录（首登强制改密）
$login = Post "/auth/login" @{ emp_no = "admin"; password = "admin123"; remember = $false } $null
Assert "超管登录" ($login.code -eq "OK")
$adminTok = $login.data.token
$me = GetReq "/me" $adminTok
Assert "首登 must_change_password=true" ($me.data.must_change_password -eq $true)
try { GetReq "/super/users" $adminTok | Out-Null; Assert "mcp 拦截（应抛错）" $false }
catch { Assert "mcp=true 拦截业务接口" ($_.Exception.Response.StatusCode.value__ -eq 403) }

# 超管改密
$cp = Put "/my/password" @{ old_password = "admin123"; new_password = "Admin2026!" } $adminTok
Assert "超管改密返回新 token" ($cp.code -eq "OK" -and $cp.data.token -ne "")
$adminTok = $cp.data.token
$me2 = GetReq "/me" $adminTok
Assert "改密后 mcp=false" ($me2.data.must_change_password -eq $false)

# 3. 创建食堂管理员（默认密码 123456）
$ca = Post "/super/canteen-admins" @{ emp_no = "CA_SMOKE"; name = "冒烟管理员"; department = "后勤" } $adminTok
Assert "创建食堂管理员" ($ca.code -eq "OK")
$caLogin = Post "/auth/login" @{ emp_no = "CA_SMOKE"; password = "123456"; remember = $false } $null
$caTok = $caLogin.data.token
Put "/my/password" @{ old_password = "123456"; new_password = "CaSmoke2026!" } $caTok | Out-Null
$caLogin2 = Post "/auth/login" @{ emp_no = "CA_SMOKE"; password = "CaSmoke2026!"; remember = $false } $null
$caTok = $caLogin2.data.token
Assert "管理员登录（改密后）" ($caLogin2.code -eq "OK")

# 4. 管理员添加员工
$emp = Post "/admin/employees" @{ emp_no = "E_SMOKE"; name = "冒烟员工"; department = "生产部" } $caTok
Assert "添加员工" ($emp.code -eq "OK")

# 5. 员工登录（首登）→ 改密
$empLogin = Post "/auth/login" @{ emp_no = "E_SMOKE"; password = "123456"; remember = $true } $null
$empTok = $empLogin.data.token
$empCp = Put "/my/password" @{ old_password = "123456"; new_password = "EmpSmoke2026!" } $empTok
$empTok = $empCp.data.token
Assert "员工改密" ($empCp.code -eq "OK")

# 6. 管理员发布明天午餐菜单
$tomorrow = (Get-Date).AddDays(1).ToString("yyyy-MM-dd")
$pub = Post "/admin/menus" @{ items = @(@{ menu_date = $tomorrow; meal = "lunch"; dishes = @(
    @{ dish_name = "红烧排骨"; taste_tags = @("微辣") }, @{ dish_name = "清炒油麦菜" }) }) } $caTok
Assert "发布菜谱（明天午餐 2 道）" ($pub.code -eq "OK" -and $pub.data.success_count -eq 2)

# 7. 游客查询
$menus = GetReq "/menus?start=$tomorrow&end=$tomorrow" $null
Assert "游客查询菜谱（2 道）" ($menus.data.dishes.Count -eq 2)

# 8. 员工报名
$signup = Post "/signups" @{ menu_date = $tomorrow; meal = "lunch" } $empTok
Assert "员工报名" ($signup.code -eq "OK")

# 9. 员工取消后重报
Invoke-RestMethod -Method Delete -Uri "$Base/signups?menu_date=$tomorrow&meal=lunch" -Headers @{ Authorization = "Bearer $empTok" } | Out-Null
$signup2 = Post "/signups" @{ menu_date = $tomorrow; meal = "lunch" } $empTok
Assert "取消后重新报名" ($signup2.code -eq "OK")

# 10. 当日不可报名
$today = (Get-Date).ToString("yyyy-MM-dd")
try { Post "/signups" @{ menu_date = $today; meal = "lunch" } $empTok | Out-Null; Assert "当日报名应被拒" $false }
catch { Assert "当日报名被拒（00:00 截止）" $true }

# 11. 管理员核销前查询（明日 → 应拒绝）
try { GetReq "/admin/verify/lookup?emp_no=E_SMOKE&menu_date=$tomorrow&meal=lunch" $caTok | Out-Null; Assert "未来日期 lookup 应拒绝" $false }
catch { Assert "防提前核销未来餐次" $true }

# 12. 统计
$stats = GetReq "/admin/stats/signups?date=$tomorrow" $caTok
$lunchCount = ($stats.data.signups | Where-Object { $_.meal -eq "lunch" }).count
Assert "报名统计（午餐 1 人）" ($lunchCount -eq 1)

# 13. 员工点评校验（未就餐应拒）
try { Post "/reviews" @{ menu_date = $tomorrow; meal = "lunch"; dish_name = "红烧排骨"; rating = 1 } $empTok | Out-Null; Assert "未就餐点评应拒" $false }
catch { Assert "未就餐不可点评" $true }

# 14. 员工提建议 + 管理员处理
$sug = Post "/suggestions" @{ content = "冒烟测试：想吃锅包肉" } $empTok
Assert "提交建议" ($sug.code -eq "OK")
$sugs = GetReq "/admin/suggestions" $caTok
$target = $sugs.data | Where-Object { $_.content -like "*冒烟*" } | Select-Object -First 1
$marked = Invoke-RestMethod -Method Patch -Uri "$Base/admin/suggestions/$($target.id)" -Headers @{ "Content-Type" = "application/json"; "Authorization" = "Bearer $caTok" } -Body '{"status":"adopted"}'
Assert "管理员标记建议" ($marked.code -eq "OK")

# 15. 超管看板
$trend = GetReq "/super/dashboard/trend" $adminTok
Assert "看板-就餐趋势" ($trend.code -eq "OK")
$viol = GetReq "/super/violations" $adminTok
Assert "违约名单" ($viol.code -eq "OK")

# 16. 越界发布整批拒绝
$bad = (Get-Date).AddDays(30).ToString("yyyy-MM-dd")
try { Post "/admin/menus" @{ items = @(@{ menu_date = $bad; meal = "lunch"; dishes = @(@{ dish_name = "X" }) }) } $caTok | Out-Null; Assert "越界发布应拒绝" $false }
catch { Assert "发布范围越界整批拒绝" $true }

Write-Host ""
Write-Host "=== 冒烟结果: PASS=$pass FAIL=$fail ==="
if ($fail -gt 0) { exit 1 }
