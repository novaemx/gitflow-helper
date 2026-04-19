$ErrorActionPreference = 'Stop'

$toolsDir    = "$(Split-Path -parent $MyInvocation.MyCommand.Definition)"
$packageName = 'gitflow-helper'
$version     = '0.5.38'

$url      = "https://github.com/novaemx/gitflow-helper/releases/download/v0.5.38/gitflow-0.5.38-windows-amd64.zip"
$checksum = 'c589f6efdb961d831928ca8ef96b6f58c1fdd8b95003f0657f2c95f673269e8a'

Install-ChocolateyZipPackage `
  -PackageName $packageName `
  -Url $url `
  -UnzipLocation $toolsDir `
  -Checksum $checksum `
  -ChecksumType 'sha256'

# Chocolatey auto-creates a shim for any .exe in the tools directory.
# The archive contains gitflow.exe which will be shimmed as 'gitflow' in PATH.
# Verify the binary landed correctly.
$binary = Join-Path $toolsDir 'gitflow.exe'
if (-not (Test-Path $binary)) {
  # Some archives nest files in a subdirectory — move it up
  $nested = Get-ChildItem -Path $toolsDir -Recurse -Filter 'gitflow.exe' | Select-Object -First 1
  if ($nested) {
    Move-Item -Path $nested.FullName -Destination $binary -Force
  } else {
    throw "gitflow.exe not found after extraction"
  }
}
