$ErrorActionPreference = "Stop"
$out = @()
try {
    $login = Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/v1/auth/login" -ContentType "application/json" -Body '{"emp_no":"E_SMOKE","password":"EmpSmoke2026!"}'
    $tok = $login.data.token
    $out += "LOGIN OK, token_len=" + $tok.Length
    $r = Invoke-WebRequest -Uri "http://localhost:8080/api/v1/my/signups" -Headers @{ Authorization = "Bearer " + $tok } -UseBasicParsing
    $out += "SIGNUPS status=" + $r.StatusCode + " body_head=" + $r.Content.Substring(0, [Math]::Min(150, $r.Content.Length))
} catch {
    $out += "ERROR: " + $_.Exception.Message
    if ($_.Exception.Response) {
        $sr = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $out += "BODY: " + $sr.ReadToEnd()
    }
}
$out | Out-File -FilePath "d:\Java\codebuddy\canteen-system\diag_result.txt" -Encoding utf8
