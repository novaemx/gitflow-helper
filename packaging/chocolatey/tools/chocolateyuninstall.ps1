$ErrorActionPreference = 'Stop'

$toolsDir = "$(Split-Path -parent $MyInvocation.MyCommand.Definition)"
$binary   = Join-Path $toolsDir 'gitflow.exe'

if (Test-Path $binary) {
  Remove-Item -Path $binary -Force
}
