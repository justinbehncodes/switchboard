# Builds the Chocolatey package for a published GitHub release.
#   .\pack.ps1 -Version 1.0.0
# Then: choco push switchboard.1.0.0.nupkg --source https://push.chocolatey.org/
param([Parameter(Mandatory)][string]$Version)

$ErrorActionPreference = 'Stop'
$here = Split-Path -Parent $MyInvocation.MyCommand.Definition

$url = "https://github.com/justinbehncodes/switchboard/releases/download/v$Version/switchboard.exe"
Write-Host "Fetching checksum for $url"
$tmp = Join-Path $env:TEMP "switchboard-$Version.exe"
Invoke-WebRequest -Uri $url -OutFile $tmp
$checksum = (Get-FileHash $tmp -Algorithm SHA256).Hash.ToLower()

$stage = Join-Path $env:TEMP "choco-switchboard"
Remove-Item $stage -Recurse -Force -ErrorAction SilentlyContinue
Copy-Item $here $stage -Recurse
Remove-Item (Join-Path $stage 'pack.ps1')

(Get-Content (Join-Path $stage 'switchboard.nuspec') -Raw) -replace '0\.0\.0', $Version |
  Set-Content (Join-Path $stage 'switchboard.nuspec')
$install = Join-Path $stage 'tools\chocolateyinstall.ps1'
(Get-Content $install -Raw) -replace '__VERSION__', $Version -replace '__CHECKSUM__', $checksum |
  Set-Content $install

choco pack (Join-Path $stage 'switchboard.nuspec') --outputdirectory $here
Write-Host "Packed. Test with: choco install switchboard -s $here -y"
