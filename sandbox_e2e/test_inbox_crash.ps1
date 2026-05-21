$ErrorActionPreference = 'Stop'

Write-Host "Killing any running test_server or localledger processes..."
Stop-Process -Name "test_server" -ErrorAction SilentlyContinue
Stop-Process -Name "localledger" -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1

# Create a custom temp workspace
$tempParent = $env:TEMP
$randomId = [System.Guid]::NewGuid().ToString()
$customWorkspace = Join-Path $tempParent "LocalLedger_CrashTest_$randomId"
New-Item -ItemType Directory -Force -Path $customWorkspace | Out-Null
$inboxPath = Join-Path $customWorkspace "inbox"
New-Item -ItemType Directory -Force -Path $inboxPath | Out-Null

# Write a dummy orphaned PDF file to simulate a crash (file exists on disk but not in DB)
$dummyFilePath = Join-Path $inboxPath "test_orphan_123.pdf"
"Dummy PDF Content" | Out-File -FilePath $dummyFilePath -Encoding utf8

Write-Host "Starting Go Server with custom workspace to trigger Orphan Reconciliation..."
$serverJob = Start-Process -FilePath "go" -ArgumentList "run ./cmd/localledger serve --workspace `"$customWorkspace`"" -NoNewWindow -PassThru

# Wait for startup and reconciliation log
Start-Sleep -Seconds 12

# Read log file to verify orphan detection
$logPath = Join-Path $customWorkspace "LocalLedger.log"
if (-not (Test-Path $logPath)) {
    # If log file is not yet written, stop server and throw
    Stop-Process -Id $serverJob.Id -ErrorAction SilentlyContinue
    Remove-Item -Recurse -Force $customWorkspace -ErrorAction SilentlyContinue
    throw "Log file not created at $logPath!"
}

$logContent = Get-Content $logPath -Raw
Write-Host "--- SERVER LOG ---"
Write-Host $logContent
Write-Host "------------------"

# Assertions
$foundOrphanWarning = $false
if ($logContent -match "Orphaned file found on disk: test_orphan_123.pdf") {
    $foundOrphanWarning = $true
}

$foundOrphanSummary = $false
if ($logContent -match "Reconciliation complete. 1 orphans found") {
    $foundOrphanSummary = $true
}

# Clean up
Stop-Process -Id $serverJob.Id -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1
Remove-Item -Recurse -Force $customWorkspace -ErrorAction SilentlyContinue

if (-not $foundOrphanWarning) {
    throw "Crash Test Assert Failed: Server did not log warning for orphaned file 'test_orphan_123.pdf'!"
}
if (-not $foundOrphanSummary) {
    throw "Crash Test Assert Failed: Server summary did not report exactly 1 orphan!"
}

Write-Host "CRASH TEST COMPLETED SUCCESSFULLY! Inbox Orphan Reconciliation detected and reported orphaned files flawlessly!"
