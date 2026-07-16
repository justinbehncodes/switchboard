$ErrorActionPreference = 'Stop'

# __VERSION__ and __CHECKSUM__ are stamped by pack.ps1 from the release.
$packageArgs = @{
  PackageName  = 'switchboard'
  FileFullPath = Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Definition) 'switchboard.exe'
  Url64bit     = 'https://github.com/justinbehncodes/switchboard/releases/download/v__VERSION__/switchboard.exe'
  Checksum64   = '__CHECKSUM__'
  ChecksumType64 = 'sha256'
}
Get-ChocolateyWebFile @packageArgs

Write-Host ""
Write-Host "Switchboard installed. Two steps to go live:" -ForegroundColor Green
Write-Host "  1. Run: switchboard install    (registers it as a browser, per-user)"
Write-Host "  2. Pick Switchboard in the Settings page that opens (Set default)."
