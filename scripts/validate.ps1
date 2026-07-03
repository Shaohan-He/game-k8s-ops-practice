$ErrorActionPreference = "Stop"

$RootDir = Split-Path -Parent $PSScriptRoot

function Require-Command {
    param([Parameter(Mandatory = $true)][string]$Name)

    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "missing required command: $Name"
    }
}

function Invoke-KustomizeRender {
    param([Parameter(Mandatory = $true)][string]$Path)

    Write-Host "[validate] kubectl kustomize $Path"
    kubectl kustomize (Join-Path $RootDir $Path) | Out-Null
}

Require-Command python
Require-Command go
Require-Command kubectl

Push-Location $RootDir
try {
    Write-Host "[validate] python -m compileall"
    python -m compileall -q common services

    Write-Host "[validate] go test ./..."
    if (-not $env:GOCACHE) {
        $env:GOCACHE = Join-Path $RootDir ".go-cache"
    }
    if (-not $env:GOMODCACHE) {
        $env:GOMODCACHE = Join-Path $RootDir ".gomodcache"
    }
    $originalAppData = $env:APPDATA
    $env:APPDATA = Join-Path $RootDir ".go-appdata"
    Push-Location (Join-Path $RootDir "operator")
    try {
        go test ./...
    }
    finally {
        Pop-Location
        $env:APPDATA = $originalAppData
    }

    Invoke-KustomizeRender "k8s"
    Invoke-KustomizeRender "operator/config/default"
    Invoke-KustomizeRender "k8s-v2"

    Write-Host "[validate] all checks passed"
}
finally {
    Pop-Location
}