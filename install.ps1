# Installer for claude-profile (Windows).
#
#   irm https://raw.githubusercontent.com/victorRadu/claude-profile/main/install.ps1 | iex
#
# From a checkout, contributors can install their local build instead:
#
#   .\install.ps1 -Local
#
# Environment:
#   CLAUDE_PROFILE_INSTALL_DIR  Install destination (default: %LOCALAPPDATA%\Programs\claude-profile)
#   CLAUDE_PROFILE_VERSION      Version to install (default: latest)
param([switch]$Local)
$ErrorActionPreference = "Stop"

$repo = "victorRadu/claude-profile"
$installDir = if ($env:CLAUDE_PROFILE_INSTALL_DIR) { $env:CLAUDE_PROFILE_INSTALL_DIR }
              else { Join-Path $env:LOCALAPPDATA "Programs\claude-profile" }
$version = if ($env:CLAUDE_PROFILE_VERSION) { $env:CLAUDE_PROFILE_VERSION } else { "latest" }

$tmp = Join-Path ([IO.Path]::GetTempPath()) ([IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    if ($Local) {
        # --- build from this checkout ---
        if (-not $PSScriptRoot -or -not (Test-Path (Join-Path $PSScriptRoot "main.go"))) {
            throw "-Local must be run from a claude-profile checkout"
        }
        if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
            throw "-Local requires the Go toolchain (https://go.dev/dl)"
        }
        $ver = try { (git -C $PSScriptRoot describe --tags --always 2>$null) } catch { "dev" }
        if (-not $ver) { $ver = "dev" }
        Write-Host "Building claude-profile $ver-local from $PSScriptRoot..."
        Push-Location $PSScriptRoot
        try { go build -ldflags "-s -w -X main.version=$ver-local" -o (Join-Path $tmp "claude-profile.exe") . }
        finally { Pop-Location }
        if ($LASTEXITCODE -ne 0) { throw "go build failed" }
    } else {
        # --- download a release ---
        if ($version -eq "latest") {
            $release = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest"
            $version = $release.tag_name
        }
        $versionNum = $version.TrimStart("v")

        $arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { throw "32-bit Windows is not supported" }
        $asset = "claude-profile_${versionNum}_windows_${arch}.zip"
        $url = "https://github.com/$repo/releases/download/$version/$asset"

        Write-Host "Downloading claude-profile $version (windows/$arch)..."
        $zip = Join-Path $tmp $asset
        Invoke-WebRequest -Uri $url -OutFile $zip

        # Verify checksum
        $sums = (Invoke-WebRequest -Uri "https://github.com/$repo/releases/download/$version/checksums.txt").Content
        $expected = ($sums -split "`n" | Where-Object { $_ -match [regex]::Escape($asset) }) -split "\s+" | Select-Object -First 1
        $actual = (Get-FileHash -Algorithm SHA256 $zip).Hash.ToLower()
        if ($expected -and $actual -ne $expected.ToLower()) { throw "checksum verification failed" }

        Expand-Archive -Path $zip -DestinationPath $tmp -Force
    }

    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    Copy-Item (Join-Path $tmp "claude-profile.exe") (Join-Path $installDir "claude-profile.exe") -Force
} finally {
    Remove-Item -Recurse -Force $tmp
}

# Short shell alias, managed by the tool itself (rename: claude-profile alias <name>)
& (Join-Path $installDir "claude-profile.exe") alias claudep | Out-Null

Write-Host "Installed $installDir\claude-profile.exe (short alias: claudep)"

# Add to user PATH if missing
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$installDir", "User")
    Write-Host "Added $installDir to your user PATH. Open a new terminal to use it."
}

Write-Host ""
Write-Host "Get started:  claude-profile create frontend --from default"
