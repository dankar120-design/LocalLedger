Write-Host "Bygger LocalLedger..." -ForegroundColor Cyan

# Bygg frontend först om det skulle behövas (för framtiden)
# Just nu kör vi bara vanilla så vi går direkt på go build

go build -ldflags="-H windowsgui" -o localledger.exe ./cmd/localledger

if ($LASTEXITCODE -ne 0) {
    Write-Host "Bygget misslyckades med felkod $LASTEXITCODE." -ForegroundColor Red
    Pause
    exit $LASTEXITCODE
}

Write-Host "Kopierar sandbox_seed.sql för fristående exekvering..." -ForegroundColor Yellow
Copy-Item -Path "internal\ledger\sandbox_seed.sql" -Destination ".\sandbox_seed.sql" -Force

Write-Host "Bygget klart!" -ForegroundColor Green
Pause
