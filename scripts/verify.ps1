$ErrorActionPreference = 'Stop'
$base = 'http://localhost:8080'
$compose = 'docker compose -f docker-compose.yml -f docker-compose.demo.yml'

function Wait-Api {
    Write-Host 'Waiting for API...' -ForegroundColor Cyan
    for ($i = 0; $i -lt 90; $i++) {
        try {
            $r = Invoke-RestMethod -Uri "$base/healthz" -TimeoutSec 3
            if ($r.success) { Write-Host 'API is ready.' -ForegroundColor Green; return }
        } catch { }
        Start-Sleep -Seconds 2
    }
    throw 'API did not become ready in time'
}

function New-Conn($name, $type, $hostName, $port, $user, $pass, $db) {
    $body = @{ name = $name; type = $type; host = $hostName; port = $port; username = $user; password = $pass; database = $db } | ConvertTo-Json
    $r = Invoke-RestMethod -Method Post -Uri "$base/api/v1/connections" -ContentType 'application/json' -Body $body
    return $r.data.id
}

function Test-Conn($id) {
    $r = Invoke-RestMethod -Method Post -Uri "$base/api/v1/connections/$id/test" -ContentType 'application/json' -Body '{}'
    if (-not $r.data.success) { throw 'connection test failed' }
    Write-Host "  version: $($r.data.version), latency: $($r.data.latency_ms) ms" -ForegroundColor DarkGray
}

function Start-Migration($name, $srcId, $tgtId, $srcDb, $tgtDb, $policy) {
    $body = @{
        name = $name
        source_connection_id = $srcId
        target_connection_id = $tgtId
        source_database = $srcDb
        target_database = $tgtDb
        migration_type = 'FULL'
        target_db_policy = $policy
        created_by = 'verify'
    } | ConvertTo-Json
    $created = Invoke-RestMethod -Method Post -Uri "$base/api/v1/migrations" -ContentType 'application/json' -Body $body
    $id = $created.data.id
    Invoke-RestMethod -Method Post -Uri "$base/api/v1/migrations/$id/start" -ContentType 'application/json' -Body '{}' | Out-Null
    return $id
}

function Wait-Migration($id) {
    for ($i = 0; $i -lt 180; $i++) {
        $t = (Invoke-RestMethod -Uri "$base/api/v1/migrations/$id").data
        $pct = $t.progress
        Write-Host "  task #$id status=$($t.status) progress=${pct}% tables=$($t.tables_completed)/$($t.tables_total)" -ForegroundColor DarkGray
        if ($t.status -eq 'SUCCESS') { return $t }
        if ($t.status -eq 'FAILED') { throw "Migration #$id FAILED: $($t.error_message)" }
        if ($t.status -eq 'CANCELLED') { throw "Migration #$id CANCELLED" }
        Start-Sleep -Seconds 2
    }
    throw "Migration #$id timed out"
}

Write-Host '===== DBMove verification =====' -ForegroundColor Cyan
Write-Host 'Expects a fresh stack (docker compose ... down -v first) and demo databases running.' -ForegroundColor DarkGray
Wait-Api

Write-Host '1. Create connections' -ForegroundColor Cyan
$mysqlSource = New-Conn 'mysql-source' 'mysql' 'mysql-source' 3306 'root' 'root123' 'source_db'
$mysqlTarget = New-Conn 'mysql-target' 'mysql' 'mysql-target' 3306 'root' 'root123' ''
$pgSource    = New-Conn 'pg-source' 'postgresql' 'pg-source' 5432 'dbuser' 'dbpass123' 'source_db'
$pgTarget    = New-Conn 'pg-target' 'postgresql' 'pg-target' 5432 'dbuser' 'dbpass123' 'postgres'
Write-Host "  created connection ids: $mysqlSource, $mysqlTarget, $pgSource, $pgTarget" -ForegroundColor DarkGray

Write-Host '2. Test connections' -ForegroundColor Cyan
Test-Conn $mysqlSource
Test-Conn $mysqlTarget
Test-Conn $pgSource
Test-Conn $pgTarget

Write-Host '3. MySQL -> MySQL full migration' -ForegroundColor Cyan
$id1 = Start-Migration 'verify-mysql' $mysqlSource $mysqlTarget 'source_db' 'target_db' 'create'
$t1 = Wait-Migration $id1
Write-Host "  MySQL migration SUCCESS (tables: $($t1.tables_total), bytes: $($t1.bytes_transferred))" -ForegroundColor Green

Write-Host '4. PostgreSQL -> PostgreSQL full migration' -ForegroundColor Cyan
$id2 = Start-Migration 'verify-postgresql' $pgSource $pgTarget 'source_db' 'target_db' 'create'
$t2 = Wait-Migration $id2
Write-Host "  PostgreSQL migration SUCCESS (tables: $($t2.tables_total), bytes: $($t2.bytes_transferred))" -ForegroundColor Green

Write-Host '5. Verify data equality' -ForegroundColor Cyan
$mysqlUsers = (& cmd /c "$compose exec -T mysql-target mysql -uroot -proot123 -N -e ""SELECT COUNT(*) FROM target_db.users"" 2>NUL").Trim()
$mysqlOrders = (& cmd /c "$compose exec -T mysql-target mysql -uroot -proot123 -N -e ""SELECT COUNT(*) FROM target_db.orders"" 2>NUL").Trim()
$pgUsers = (& cmd /c "$compose exec -T pg-target psql -U dbuser -d target_db -tAc ""SELECT COUNT(*) FROM users""" 2>$null).Trim()
$pgOrders = (& cmd /c "$compose exec -T pg-target psql -U dbuser -d target_db -tAc ""SELECT COUNT(*) FROM orders""" 2>$null).Trim()

if ($mysqlUsers -ne '3' -or $mysqlOrders -ne '5') { throw "MySQL data mismatch: users=$mysqlUsers orders=$mysqlOrders" }
if ($pgUsers -ne '3' -or $pgOrders -ne '5') { throw "PostgreSQL data mismatch: users=$pgUsers orders=$pgOrders" }

Write-Host "  MySQL target: users=$mysqlUsers orders=$mysqlOrders" -ForegroundColor Green
Write-Host "  PostgreSQL target: users=$pgUsers orders=$pgOrders" -ForegroundColor Green
Write-Host '===== VERIFICATION PASSED =====' -ForegroundColor Green
