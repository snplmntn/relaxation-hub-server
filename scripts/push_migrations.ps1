param(
    [string]$DatabaseUrl = $env:DATABASE_URL,
    [string]$MigrationsDir = (Join-Path $PSScriptRoot "..\internal\db\migrations"),
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

function Resolve-DatabaseUrl {
    param([string]$Url)

    if ([string]::IsNullOrWhiteSpace($Url)) {
        return $Url
    }

    $schemeSeparator = "://"
    $schemeIndex = $Url.IndexOf($schemeSeparator)
    if ($schemeIndex -lt 0) {
        return $Url
    }

    $scheme = $Url.Substring(0, $schemeIndex + $schemeSeparator.Length)
    $authorityAndPath = $Url.Substring($schemeIndex + $schemeSeparator.Length)
    $atIndex = $authorityAndPath.LastIndexOf("@")
    if ($atIndex -lt 0) {
        return $Url
    }

    $credentials = $authorityAndPath.Substring(0, $atIndex)
    $hostPart = $authorityAndPath.Substring($atIndex + 1)
    if ([string]::IsNullOrWhiteSpace($credentials) -or [string]::IsNullOrWhiteSpace($hostPart)) {
        return $Url
    }

    $firstColon = $credentials.IndexOf(":")
    if ($firstColon -lt 0) {
        return $Url
    }

    $user = $credentials.Substring(0, $firstColon)
    $password = $credentials.Substring($firstColon + 1)
    if ([string]::IsNullOrWhiteSpace($password)) {
        return $Url
    }

    $decodedPassword = [System.Uri]::UnescapeDataString($password)
    $normalizedPassword = [System.Uri]::EscapeDataString($decodedPassword)
    if ($normalizedPassword -ceq $password) {
        return $Url
    }

    Write-Host "INFO: Normalized DATABASE_URL password encoding."
    return "${scheme}$user`:$normalizedPassword@$hostPart"
}

if ([string]::IsNullOrWhiteSpace($DatabaseUrl)) {
    throw "Database URL is required. Set DATABASE_URL or pass -DatabaseUrl."
}
$DatabaseUrl = Resolve-DatabaseUrl -Url $DatabaseUrl

$psqlPath = $null
$psql = Get-Command psql -ErrorAction SilentlyContinue
if ($psql) {
    $psqlPath = $psql.Source
} else {
    $postgresRoot = "C:\Program Files\PostgreSQL"
    if (Test-Path $postgresRoot) {
        $candidate = Get-ChildItem -Path $postgresRoot -Directory |
            Sort-Object Name -Descending |
            ForEach-Object { Join-Path $_.FullName "bin\psql.exe" } |
            Where-Object { Test-Path $_ } |
            Select-Object -First 1

        if ($candidate) {
            $psqlPath = $candidate
        }
    }
}

if (-not $psqlPath) {
    throw "psql is required but was not found in PATH or standard PostgreSQL install directories."
}

$resolvedMigrationsDir = Resolve-Path $MigrationsDir -ErrorAction Stop
$migrationFiles = Get-ChildItem -Path $resolvedMigrationsDir -Filter "*.sql" | Sort-Object Name

if ($migrationFiles.Count -eq 0) {
    Write-Host "No migrations found in $resolvedMigrationsDir"
    exit 0
}

$historyTableSQL = @"
CREATE TABLE IF NOT EXISTS public.app_migration_history (
    filename TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
"@

if (-not $DryRun) {
    & $psqlPath $DatabaseUrl -v ON_ERROR_STOP=1 -c $historyTableSQL | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to ensure app_migration_history table exists."
    }
}

$applied = 0
$skipped = 0

foreach ($file in $migrationFiles) {
    $safeName = $file.Name.Replace("'", "''")
    $checkSQL = "SELECT 1 FROM public.app_migration_history WHERE filename = '$safeName' LIMIT 1;"
    $alreadyApplied = ""

    if (-not $DryRun) {
        $checkResult = & $psqlPath $DatabaseUrl -tA -v ON_ERROR_STOP=1 -c $checkSQL
        if ($LASTEXITCODE -ne 0) {
            throw "Failed to check migration history for $($file.Name)."
        }
        if ($null -ne $checkResult) {
            $alreadyApplied = ($checkResult | Out-String).Trim()
        }
    }

    if ($alreadyApplied -eq "1") {
        Write-Host "Skipping (already applied): $($file.Name)"
        $skipped++
        continue
    }

    if ($DryRun) {
        Write-Host "Dry run - would apply: $($file.Name)"
        $applied++
        continue
    }

    Write-Host "Applying: $($file.Name)"
    & $psqlPath $DatabaseUrl -v ON_ERROR_STOP=1 -f $file.FullName | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to apply migration $($file.Name)."
    }

    $insertSQL = "INSERT INTO public.app_migration_history (filename) VALUES ('$safeName') ON CONFLICT (filename) DO NOTHING;"
    & $psqlPath $DatabaseUrl -v ON_ERROR_STOP=1 -c $insertSQL | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to record migration history for $($file.Name)."
    }
    $applied++
}

Write-Host "Done. Applied: $applied, Skipped: $skipped, Total: $($migrationFiles.Count)"
