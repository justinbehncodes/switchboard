$ErrorActionPreference = 'Stop'

# Remove the per-user browser registration before the shim goes away.
$exe = Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Definition) 'switchboard.exe'
if (Test-Path $exe) {
  & $exe uninstall
}
