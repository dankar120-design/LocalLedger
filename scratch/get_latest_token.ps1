$dirs = Get-ChildItem -Path $env:TEMP -Filter "LocalLedger_Sandbox_*" | Sort-Object LastWriteTime -Descending
if ($dirs.Count -gt 0) {
    $dir = $dirs[0].FullName
    $token = (Get-Content (Join-Path $dir ".session_token") -Raw).Trim()
    $port = (Get-Content (Join-Path $dir ".server_port") -Raw).Trim()
    Write-Output "PORT:$port"
    Write-Output "TOKEN:$token"
} else {
    Write-Output "NO_SANDBOX"
}
