# Uninstaller for claude-profile (Windows).
#
#   irm https://raw.githubusercontent.com/victorRadu/claude-profile/main/uninstall.ps1 | iex
#
# Removes the binary, the wrapper shim, the managed profile-script block and
# the PATH entries the installer added. Profile data (logins, history) is
# kept unless you confirm, or set CLAUDE_PROFILE_PURGE=1.
$ErrorActionPreference = "Stop"

$profilesDir = if ($env:CLAUDE_PROFILES_DIR) { $env:CLAUDE_PROFILES_DIR } else { Join-Path $env:USERPROFILE ".claude-profiles" }
$installDir  = if ($env:CLAUDE_PROFILE_INSTALL_DIR) { $env:CLAUDE_PROFILE_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Programs\claude-profile" }
$blockStart  = "# >>> claude-profile >>>"
$blockEnd    = "# <<< claude-profile <<<"

# --- managed block in PowerShell profiles -----------------------------------
$profileFiles = @(
    (Join-Path $env:USERPROFILE "Documents\PowerShell\Microsoft.PowerShell_profile.ps1"),
    (Join-Path $env:USERPROFILE "Documents\WindowsPowerShell\Microsoft.PowerShell_profile.ps1")
)
foreach ($file in $profileFiles) {
    if (-not (Test-Path $file)) { continue }
    $lines = Get-Content $file
    if ($lines -notcontains $blockStart) { continue }
    $kept = New-Object System.Collections.Generic.List[string]
    $inBlock = $false
    foreach ($line in $lines) {
        if ($line.Trim() -eq $blockStart) { $inBlock = $true; continue }
        if ($line.Trim() -eq $blockEnd)   { $inBlock = $false; continue }
        if (-not $inBlock) { $kept.Add($line) }
    }
    Set-Content -Path $file -Value $kept
    Write-Host "Removed managed block from $file"
}

# --- wrapper shim -------------------------------------------------------------
$shim = Join-Path $profilesDir ".bin\claude.cmd"
if (Test-Path $shim) {
    Remove-Item $shim -Force
    Remove-Item (Join-Path $profilesDir ".bin") -Force -ErrorAction SilentlyContinue
    Write-Host "Removed the claude wrapper"
}

# --- PATH entries the installer added ------------------------------------------
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath) {
    $cleaned = ($userPath -split ";" | Where-Object {
        $_ -and $_ -ne $installDir -and $_ -ne (Join-Path $profilesDir ".bin")
    }) -join ";"
    if ($cleaned -ne $userPath) {
        [Environment]::SetEnvironmentVariable("Path", $cleaned, "User")
        Write-Host "Removed claude-profile entries from your user PATH"
    }
}

# --- binary --------------------------------------------------------------------
if (Test-Path $installDir) {
    Remove-Item -Recurse -Force $installDir
    Write-Host "Removed $installDir"
}

# --- profile data (opt-in) --------------------------------------------------------
if (Test-Path $profilesDir) {
    $purge = "n"
    if ($env:CLAUDE_PROFILE_PURGE -eq "1") { $purge = "y" }
    elseif ([Environment]::UserInteractive) {
        $purge = Read-Host "Also delete all profile data in $profilesDir? This includes logins and history. [y/N]"
    }
    if ($purge -match "^(y|Y|yes)$") {
        Remove-Item -Recurse -Force $profilesDir
        Write-Host "Deleted $profilesDir"
    } else {
        Write-Host "Kept profile data at $profilesDir - delete it manually when ready."
    }
}

Write-Host ""
Write-Host "Note: .claude-profile binding files inside your project folders are not touched."
Write-Host "claude-profile has been uninstalled. Open a new terminal to apply."
