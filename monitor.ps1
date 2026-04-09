$MAX_WAIT = 900
$ELAPSED = 0
Write-Host "[INFO] Build tamamlanmasini bekleniyor..." -ForegroundColor Cyan
while ($ELAPSED -lt $MAX_WAIT) {
    $ps = docker-compose ps 2>&1
    $count = ($ps | Select-String "Up" | Measure-Object).Count
    if ($count -ge 3) {
        Write-Host "[OK] Servisler baslamis! Tarayicida aciliyor..." -ForegroundColor Green
        Start-Sleep 2
        Start-Process "http://localhost:8080"
        exit 0
    }
    Write-Host "[..] Bekleniyor... ($ELAPSED`s)" -ForegroundColor Yellow
    Start-Sleep 5
    $ELAPSED += 5
}
Write-Host "[ERROR] Timeout!" -ForegroundColor Red
