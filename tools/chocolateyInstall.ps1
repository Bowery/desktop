$packageName = "bowery"
$url = "http://desktop.bowery.io.s3.amazonaws.com/{{version}}_windows_386_noinstaller.zip"
$url64 = "http://desktop.bowery.io.s3.amazonaws.com/{{version}}_windows_amd64_noinstaller.zip"

try {
  # Unzips and installs to pkg path, and adds links for binaries to a PATH directory.
  $root = "$(Split-Path -parent $MyInvocation.MyCommand.Definition)"
  $chocTempDir = Join-Path $env:TEMP "chocolatey"
  $tempDir = Join-Path $chocTempDir "$packageName"
  New-Item "$tempDir" -Force -ItemType directory | Out-Null
  $file = Join-Path $tempDir "$($packageName)Install.zip"
  Get-ChocolateyWebFile "$packageName" "$file" "$url" "$url64"
  Get-ChocolateyUnzip "$file" "$root"

  # Write the shortcut to the Desktop.
  $desktop = $([System.Environment]::GetFolderPath([System.Environment+SpecialFolder]::DesktopDirectory))
  $shell = New-Object -com "WScript.Shell"
  $lnk = $shell.CreateShortcut("$desktop\Bowery.lnk")
  $lnk.TargetPath = "$root\bowery.exe"
  $lnk.IconLocation = "$root\logo.ico"
  $lnk.Save()
  Write-Host "Created shortcut here $desktop\Bowery.lnk"

  & "$root\rmmanifest.exe" "$root\bowery.exe"
  Write-ChocolateySuccess "$packageName"
} catch {
  Write-ChocolateyFailure "$packageName" "$($_.Exception.Message)"
  throw
}
