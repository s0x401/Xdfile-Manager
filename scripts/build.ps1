$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

function Invoke-Native {
    param(
        [Parameter(Mandatory = $true)]
        [string[]]$Command
    )

    & $Command[0] $Command[1..($Command.Length - 1)]
    if ($LASTEXITCODE -ne 0) {
        throw ("Command failed with exit code {0}: {1}" -f $LASTEXITCODE, ($Command -join " "))
    }
}

$targets = @(
    @{ GOOS = "windows"; GOARCH = "amd64"; Output = "dist/windows-amd64/xdfile.exe" },
    @{ GOOS = "darwin"; GOARCH = "amd64"; Output = "dist/darwin-amd64/xdfile" },
    @{ GOOS = "darwin"; GOARCH = "arm64"; Output = "dist/darwin-arm64/xdfile" }
)

Invoke-Native @("go", "test", "./...")

foreach ($target in $targets) {
    $outputPath = Join-Path $root $target.Output
    $outputDir = Split-Path -Parent $outputPath
    New-Item -ItemType Directory -Force -Path $outputDir | Out-Null

    Write-Host ("Building {0}/{1} -> {2}" -f $target.GOOS, $target.GOARCH, $target.Output)
    $env:GOOS = $target.GOOS
    $env:GOARCH = $target.GOARCH
    Invoke-Native @("go", "build", "-o", $outputPath, ".")
}

Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
