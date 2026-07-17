$ErrorActionPreference = 'Stop'

# Remove the per-user browser registration before the shim goes away.
$exe = Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Definition) 'switchboard.exe'
if (Test-Path $exe) {
  # switchboard.exe is a GUI-subsystem binary, so PowerShell's call operator
  # would not wait for it. Run it hidden and wait for exit so Chocolatey
  # doesn't remove the package folder while the process is still running.
  $p = Start-Process -FilePath $exe -ArgumentList 'uninstall' -WindowStyle Hidden -PassThru
  if (-not $p.WaitForExit(30000)) {
    $p.Kill()
    Write-Warning "switchboard uninstall timed out; per-user registry cleanup may be incomplete."
  }
}
