
$RootPath = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location -Path $RootPath

$Port = 8877
$Url = http://localhost:$Port"

Write-Host ========================================" -ForegroundColor Cyan
Write-Host     SNJSYNC - Web Lokal" -ForegroundColor Cyan
Write-Host ========================================" -ForegroundColor Cyan
Write-Host "

$ExePath = Join-Path $RootPath snjsync.exe"


if (-not (Test-Path $ExePath)) {
    Write-Host [ERROR] snjsync.exe tidak ditemukan!" -ForegroundColor Red
    Write-Host Mencoba build ulang..." -ForegroundColor Yellow
    Write-Host "

    try {
        & go build -o snjsync.exe .
        if ($LASTEXITCODE -ne 0) {
            throw Build gagal dengan exit code $LASTEXITCODE"
        }
        Write-Host [OK] Build berhasil!" -ForegroundColor Green
        Write-Host "
    }
    catch {
        Write-Host [ERROR] $($_.Exception.Message)" -ForegroundColor Red
        Read-Host Tekan Enter untuk keluar"
        exit 1
    }
}


$PortInUse = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
if ($PortInUse) {
    $OwnerPid = $PortInUse.OwningProcess
    Write-Host [INFO] Port $Port sudah digunakan" -ForegroundColor Yellow
    Write-Host PID: $OwnerPid" -ForegroundColor Gray
    Write-Host Mungkin SNJSYNC sudah berjalan..." -ForegroundColor Yellow
    Write-Host "
    Write-Host Membuka browser..." -ForegroundColor Cyan
    
    
    
    Start-Process $Url/?r=$([guid]::NewGuid().ToString('N').Substring(0,8))"
    Read-Host Tekan Enter untuk keluar"
    exit 0
}

Write-Host [START] Menjalankan SNJSYNC..." -ForegroundColor Green
Write-Host Root: $RootPath" -ForegroundColor Gray
Write-Host URL:  $Url" -ForegroundColor Cyan
Write-Host "
Write-Host Browser akan terbuka otomatis. Tekan Ctrl+C untuk menghentikan server." -ForegroundColor Yellow
Write-Host "


& .\snjsync.exe

Write-Host "
Write-Host Server berhenti." -ForegroundColor Yellow
Read-Host Tekan Enter untuk keluar"
