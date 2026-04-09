#!/usr/bin/env pwsh
<#
.SYNOPSIS
Docker containerlarin baslanmasini bekle ve tarayicida ac
#>

$MAX_WAIT_SECONDS = 900  # 15 dakika
$CHECK_INTERVAL = 5
$ELAPSED = 0

Write-Host "[INFO] Vantage servislerin baslanmasini bekleniyor..." -ForegroundColor Cyan
Write-Host "       Timeout: $($MAX_WAIT_SECONDS)s" -ForegroundColor Gray

while ($ELAPSED -lt $MAX_WAIT_SECONDS) {
    try {
        # Check if all services are healthy
        $ps_output = & docker-compose ps 2>&1
        $vantage_up = $ps_output | Select-String "vantage-core.*Up" | Measure-Object | Select-Object -ExpandProperty Count
        $caddy_up = $ps_output | Select-String "vantage-caddy.*Up" | Measure-Object | Select-Object -ExpandProperty Count
        $postfix_up = $ps_output | Select-String "vantage-postfix.*Up" | Measure-Object | Select-Object -ExpandProperty Count

        if ($vantage_up -eq 1 -and $caddy_up -eq 1 -and $postfix_up -eq 1) {
            Write-Host "[OK] Tum servisler baslamis!" -ForegroundColor Green
            Write-Host ""
            
            # Wait a bit for the app to be ready
            Write-Host "[INFO] Vantage uygulamasinin hazir olmasini bekleniyor..." -ForegroundColor Yellow
            Start-Sleep -Seconds 3
            
            # Try to connect to the service
            try {
                $response = Invoke-WebRequest -Uri "http://localhost:8080" -Method GET -TimeoutSec 5
                if ($response.StatusCode -eq 200 -or $response.StatusCode -eq 302) {
                    Write-Host "[OK] Uygulama yanit veriyor!" -ForegroundColor Green
                    Write-Host ""
                    Write-Host "[INFO] Servis Bilgileri:" -ForegroundColor Cyan
                    Write-Host "  - Vantage Admin: http://localhost:8080/admin" -ForegroundColor White
                    Write-Host "  - Vantage API: http://localhost:8080/api" -ForegroundColor White
                    Write-Host "  - Phishing Server: http://localhost/phish" -ForegroundColor White
                    Write-Host ""
                    Write-Host "[INFO] Default Credentials:" -ForegroundColor Yellow
                    Write-Host "  - Username: admin" -ForegroundColor Gray
                    Write-Host "  - Password: gophish" -ForegroundColor Gray
                    
                    # Open in browser
                    Write-Host ""
                    Write-Host "[INFO] Tarayicida aciliyor..." -ForegroundColor Cyan
                    Start-Process "http://localhost:8080"
                    exit 0
                }
            } catch {
                Write-Host "       Tekrar deneniyor..." -ForegroundColor Gray
                Start-Sleep -Seconds 2
            }
            
        } else {
            Write-Host "[$ELAPSED`s] Vantage: $(if ($vantage_up) {'OK'} else {'WAIT'}) | Caddy: $(if ($caddy_up) {'OK'} else {'WAIT'}) | Postfix: $(if ($postfix_up) {'OK'} else {'WAIT'})" -ForegroundColor Gray
        }
    } catch {
        Write-Host "[$ELAPSED`s] [WARN] Docker kontrol hatasi... (Devam ediliyor)" -ForegroundColor DarkYellow
    }

    Start-Sleep -Seconds $CHECK_INTERVAL
    $ELAPSED += $CHECK_INTERVAL
}

Write-Host ""
Write-Host "[ERROR] Timeout! Servisler $MAX_WAIT_SECONDS saniye icinde baslamadi." -ForegroundColor Red
Write-Host ""
Write-Host "[INFO] Hata Ayiklama:" -ForegroundColor Yellow
Write-Host "  1. Docker Desktop'in calistigini kontrol edin" -ForegroundColor Gray
Write-Host "  2. Log'lari kontrol edin: docker-compose logs" -ForegroundColor Gray
Write-Host "  3. Build'i kontrol edin: docker-compose build --no-cache" -ForegroundColor Gray
exit 1
