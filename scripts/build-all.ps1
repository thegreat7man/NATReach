$ErrorActionPreference = "Stop"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go 1.25+ is required: https://go.dev/dl/"
}

$Version = if ($env:VERSION) { $env:VERSION } else { "0.1.0" }
$Root = Split-Path -Parent $PSScriptRoot
$Dist = Join-Path $Root "dist"
New-Item -ItemType Directory -Force -Path $Dist | Out-Null
Remove-Item (Join-Path $Dist "natreach-$Version-*.tar.gz") -Force -ErrorAction SilentlyContinue
Remove-Item (Join-Path $Dist "natreach-$Version-*.zip") -Force -ErrorAction SilentlyContinue
Remove-Item (Join-Path $Dist "SHA256SUMS") -Force -ErrorAction SilentlyContinue

$Targets = @(
    @("darwin", "amd64", ""), @("darwin", "arm64", ""),
    @("linux", "amd64", ""), @("linux", "arm64", ""),
    @("windows", "amd64", ".exe"), @("windows", "arm64", ".exe")
)

foreach ($Target in $Targets) {
    $OS, $Arch, $Ext = $Target
	$Name = "natreach-$Version-$OS-$Arch"
    $Stage = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
    New-Item -ItemType Directory -Path $Stage | Out-Null
	$Binary = Join-Path $Stage "natreach$Ext"
    Write-Host "Building $Name"
    $env:CGO_ENABLED = "0"
    $env:GOOS = $OS
    $env:GOARCH = $Arch
    Push-Location $Root
    try { go build -trimpath -ldflags "-s -w -X main.version=$Version" -o $Binary . } finally { Pop-Location }
    Copy-Item (Join-Path $Root "README.md"), (Join-Path $Root "LICENSE") -Destination $Stage
    if ($OS -eq "darwin") {
		Copy-Item (Join-Path $Root "assets/NATReach.command") -Destination $Stage
    }
    Compress-Archive -Path (Join-Path $Stage "*") -DestinationPath (Join-Path $Dist "$Name.zip") -Force
    Remove-Item -Recurse -Force $Stage
}

Get-ChildItem -Path $Dist -File | Where-Object {
	$_.Name -like "natreach-$Version-*.zip" -or $_.Name -like "natreach-$Version-*.tar.gz"
} | Sort-Object Name | ForEach-Object {
    $Hash = (Get-FileHash -Algorithm SHA256 $_.FullName).Hash.ToLowerInvariant()
    "$Hash  $($_.Name)"
} | Set-Content -Encoding ascii (Join-Path $Dist "SHA256SUMS")

Write-Host "Done: $Dist"
