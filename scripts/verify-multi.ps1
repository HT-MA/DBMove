$ErrorActionPreference = 'Stop'
$base = 'http://localhost:8080'
$compose = 'docker compose -f docker-compose.yml -f docker-compose.demo.yml'

function Wait-Api {
    Write-Host 'Waiting for API...' -ForegroundColor Cyan
    for ($i = 0; $i -lt 90; $i++) {
        try {
            $r = Invoke-RestMethod -Uri "$base/healthz" -TimeoutSec 3
            if ($r.success) { return }
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

function Start-MultiMigration($name, $srcId, $tgtId, $pairs, $policy) {
    $body = @{
        name = $name
        source_connection_id = $srcId
        target_connection_id = $tgtId
        databases = $pairs
        migration_type = 'FULL'
        target_db_policy = $policy
        created_by = 'verify-multi'
    } | ConvertTo-Json -Depth 5
    $created = Invoke-RestMethod -Method Post -Uri "$base/api/v1/migrations" -ContentType 'application/json' -Body $body
    $id = $created.data.id
    Invoke-RestMethod -Method Post -Uri "$base/api/v1/migrations/$id/start" -ContentType 'application/json' -Body '{}' | Out-Null
    return $id
}

function Wait-Migration($id) {
    for ($i = 0; $i -lt 180; $i++) {
        $t = (Invoke-RestMethod -Uri "$base/api/v1/migrations/$id").data
        Write-Host "  task #$id status=$($t.status) progress=$($t.progress)% tables=$($t.tables_completed)/$($t.tables_total)" -ForegroundColor DarkGray
        if ($t.status -eq 'SUCCESS') { return $t }
        if ($t.status -eq 'FAILED') { throw "Migration #$id FAILED: $($t.error_message)" }
        if ($t.status -eq 'CANCELLED') { throw "Migration #$id CANCELLED" }
        Start-Sleep -Seconds 2
    }
    throw "Migration #$id timed out"
}

Write-Host '===== DBMove multi-database verification =====' -ForegroundColor Cyan
Write-Host 'Expects a fresh stack with demo databases (source_db + sales_db / analytics_db).' -ForegroundColor DarkGray
Wait-Api

Write-Host '1. Create connections' -ForegroundColor Cyan
$mysqlSource = New-Conn 'mysql-source' 'mysql' 'mysql-source' 3306 'root' 'root123' 'source_db'
$mysqlTarget = New-Conn 'mysql-target' 'mysql' 'mysql-target' 3306 'root' 'root123' ''
$pgSource    = New-Conn 'pg-source' 'postgresql' 'pg-source' 5432 'dbuser' 'dbpass123' 'source_db'
$pgTarget    = New-Conn 'pg-target' 'postgresql' 'pg-target' 5432 'dbuser' 'dbpass123' 'postgres'

Write-Host '2. Clean target databases for a clean create-policy run' -ForegroundColor Cyan
& cmd /c "$compose exec -T mysql-target mysql -uroot -proot123 -e ""DROP DATABASE IF EXISTS target_db; DROP DATABASE IF EXISTS sales_db2;"" 2>NUL" | Out-Null
& cmd /c "$compose exec -T pg-target psql -U dbuser -d postgres -c ""DROP DATABASE IF EXISTS target_db"" 2>NUL" | Out-Null
& cmd /c "$compose exec -T pg-target psql -U dbuser -d postgres -c ""DROP DATABASE IF EXISTS analytics_db2"" 2>NUL" | Out-Null

Write-Host '3. MySQL multi-database migration (source_db -> target_db, sales_db -> sales_db2)' -ForegroundColor Cyan
$mysqlPairs = @(
    @{ source = 'source_db'; target = 'target_db' },
    @{ source = 'sales_db'; target = 'sales_db2' }
)
$id1 = Start-MultiMigration 'verify-mysql-multi' $mysqlSource $mysqlTarget $mysqlPairs 'create'
$t1 = Wait-Migration $id1
Write-Host "  MySQL multi migration SUCCESS (databases: $($t1.databases.Count), tables: $($t1.tables_total))" -ForegroundColor Green

Write-Host '4. PostgreSQL multi-database migration (source_db -> target_db, analytics_db -> analytics_db2)' -ForegroundColor Cyan
$pgPairs = @(
    @{ source = 'source_db'; target = 'target_db' },
    @{ source = 'analytics_db'; target = 'analytics_db2' }
)
$id2 = Start-MultiMigration 'verify-postgresql-multi' $pgSource $pgTarget $pgPairs 'create'
$t2 = Wait-Migration $id2
Write-Host "  PostgreSQL multi migration SUCCESS (databases: $($t2.databases.Count), tables: $($t2.tables_total))" -ForegroundColor Green

Write-Host '5. Verify data equality' -ForegroundColor Cyan
$myUsers  = (& cmd /c "$compose exec -T mysql-target mysql -uroot -proot123 -N -e ""SELECT COUNT(*) FROM target_db.users"" 2>NUL").Trim()
$myOrders = (& cmd /c "$compose exec -T mysql-target mysql -uroot -proot123 -N -e ""SELECT COUNT(*) FROM target_db.orders"" 2>NUL").Trim()
$myProd   = (& cmd /c "$compose exec -T mysql-target mysql -uroot -proot123 -N -e ""SELECT COUNT(*) FROM sales_db2.products"" 2>NUL").Trim()
$pgUsers  = (& cmd /c "$compose exec -T pg-target psql -U dbuser -d target_db -tAc ""SELECT COUNT(*) FROM users""" 2>$null).Trim()
$pgOrders = (& cmd /c "$compose exec -T pg-target psql -U dbuser -d target_db -tAc ""SELECT COUNT(*) FROM orders""" 2>$null).Trim()
$pgEvents = (& cmd /c "$compose exec -T pg-target psql -U dbuser -d analytics_db2 -tAc ""SELECT COUNT(*) FROM events""" 2>$null).Trim()

if ($myUsers -ne '3' -or $myOrders -ne '5' -or $myProd -ne '2') { throw "MySQL multi mismatch: users=$myUsers orders=$myOrders products=$myProd" }
if ($pgUsers -ne '3' -or $pgOrders -ne '5' -or $pgEvents -ne '2') { throw "PostgreSQL multi mismatch: users=$pgUsers orders=$pgOrders events=$pgEvents" }

Write-Host "  MySQL target: users=$myUsers orders=$myOrders products=$myProd" -ForegroundColor Green
Write-Host "  PostgreSQL target: users=$pgUsers orders=$pgOrders events=$pgEvents" -ForegroundColor Green
Write-Host '===== MULTI-DATABASE VERIFICATION PASSED =====' -ForegroundColor Green
