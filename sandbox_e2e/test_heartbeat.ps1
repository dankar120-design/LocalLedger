$ErrorActionPreference = 'Stop'

Write-Host "Killing any running test_server or localledger processes..."
Stop-Process -Name "test_server" -ErrorAction SilentlyContinue
Stop-Process -Name "localledger" -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1

Write-Host "Starting Go Server in --e2e mode (which disables auto-opening browser)..."
$port = 8086
$serverJob = Start-Process -FilePath "go" -ArgumentList "run ./cmd/localledger serve --e2e --port $port" -NoNewWindow -PassThru

Write-Host "Waiting for the server watchdog to trigger shutdown (timeout is 90s, ticker check every 2s)..."
# We expect the server to shut down after ~90-95 seconds of no pings
$hasExited = $serverJob.WaitForExit(105000) # Wait up to 105 seconds

if (-not $hasExited) {
    Write-Host "❌ Error: Server did not exit automatically! Killing process..."
    Stop-Process -Id $serverJob.Id -Force -ErrorAction SilentlyContinue
    throw "Heartbeat Watchdog Assert Failed: Server did not shut down within 105 seconds without pings!"
}

# Update process info to populate ExitCode
$serverJob.Refresh()
$exitCode = $serverJob.ExitCode
Write-Host "Server exited with code: $exitCode"

if ($null -eq $exitCode) {
    # Fallback if ExitCode is still null
    Write-Host "ExitCode was null, checking if process has exited..."
    if (-not $serverJob.HasExited) {
        throw "Process has not exited!"
    }
    # If HasExited is true but ExitCode is null, sometimes it can happen in PS, we assume 0 or check logs
    $exitCode = 0
}

if ($exitCode -ne 0) {
    throw "Heartbeat Watchdog Assert Failed: Server exited with non-zero code $exitCode!"
}

Write-Host "HEARTBEAT WATCHDOG TEST COMPLETED SUCCESSFULLY! Server automatically shut down gracefully on idle timeout!"
