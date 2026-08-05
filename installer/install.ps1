<#
.SYNOPSIS
    Install irori, the TUI for running a Minecraft server.

.EXAMPLE
    irm https://raw.githubusercontent.com/BX-Team/irori/master/installer/install.ps1 | iex

.EXAMPLE
    & ([scriptblock]::Create((irm https://raw.githubusercontent.com/BX-Team/irori/master/installer/install.ps1))) -Version 1.2.3
#>
[CmdletBinding()]
param(
    [string]$Version = $(if ($env:IRORI_VERSION) { $env:IRORI_VERSION } else { 'latest' }),
    [string]$InstallDir = $(if ($env:IRORI_BIN_DIR) { $env:IRORI_BIN_DIR } else { "$env:LOCALAPPDATA\Programs\irori" })
)

$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$repo = 'BX-Team/irori'

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    default { throw "irori: unsupported architecture: $env:PROCESSOR_ARCHITECTURE (irori needs a 64-bit system)" }
}

$asset = "irori-windows-$arch.zip"
$base = if ($Version -eq 'latest') {
    "https://github.com/$repo/releases/latest/download"
} elseif ($Version.StartsWith('v')) {
    "https://github.com/$repo/releases/download/$Version"
} else {
    "https://github.com/$repo/releases/download/v$Version"
}

$tmp = Join-Path ([IO.Path]::GetTempPath()) ("irori-" + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tmp -Force | Out-Null

try {
    $zip = Join-Path $tmp $asset
    Write-Host "irori: downloading $asset ($Version)"
    Invoke-WebRequest -Uri "$base/$asset" -OutFile $zip -UseBasicParsing

    # The checksum file is best effort: an old release that predates it should
    # not block an install, but a mismatch always must.
    try {
        $sums = Join-Path $tmp "$asset.sha256"
        Invoke-WebRequest -Uri "$base/$asset.sha256" -OutFile $sums -UseBasicParsing
        $want = (Get-Content $sums -Raw).Trim()
        $got = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
        if ($want -ne $got) {
            throw "irori: checksum mismatch: expected $want, got $got"
        }
        Write-Host 'irori: checksum ok'
    } catch [Net.WebException] {
        Write-Warning 'irori: no checksum published for this release, skipping verification'
    }

    Expand-Archive -Path $zip -DestinationPath $tmp -Force
    $exe = Join-Path $tmp 'irori.exe'
    if (-not (Test-Path $exe)) { throw 'irori: archive did not contain irori.exe' }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Copy-Item $exe (Join-Path $InstallDir 'irori.exe') -Force
    Write-Host "irori: installed to $InstallDir\irori.exe"

    # Persist for future shells, and patch the current one so `irori` works now.
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($userPath -notlike "*$InstallDir*") {
        $joined = if ([string]::IsNullOrEmpty($userPath)) { $InstallDir } else { "$userPath;$InstallDir" }
        [Environment]::SetEnvironmentVariable('Path', $joined, 'User')
        Write-Host "irori: added $InstallDir to your user PATH"
    }
    if ($env:Path -notlike "*$InstallDir*") { $env:Path = "$env:Path;$InstallDir" }

    & (Join-Path $InstallDir 'irori.exe') --version
} finally {
    Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}
