# Usage: powershell -ExecutionPolicy unrestricted -File .\release_client.ps1
#        .\release_client.ps1(while in powershell cli)
$ErrorActionPreference = "Stop"
$origpwd = $pwd

# Get and cd into the directory containing this script.
$cmd = $MyInvocation.MyCommand
$root = $cmd.Definition.Replace($cmd.Name, "..")

# Retrieve the version.
$version = Get-Content VERSION
echo "Version: $version"

# Pack the choco pkg.
New-Item "$root\choco\tools" -Force -ItemType directory
New-Item "$root\choco\bowery.nuspec" -Force -ItemType file -Value ""
$nuspec = (Get-Content "$root\bowery.nuspec") -replace "{{version}}", "$version"
[System.IO.File]::WriteAllLines("$root\choco\bowery.nuspec", $nuspec)
ForEach ($file in Get-ChildItem "$root\tools") {
  $content = (Get-Content "$root\tools\$($file.name)") -replace "{{version}}", "$version"
  [System.IO.File]::WriteAllLines("$root\choco\tools\$($file.name)", $content)
}
cd "$root\choco"
cpack
if ($LastExitCode -gt 0) {
  cd $origpwd
  exit 1
}

# Push the choco pkg.
cinst nuget.commandline
if ($LastExitCode -gt 0) {
  cd $origpwd
  exit 1
}
nuget setApiKey "2e664545-c457-4d43-afc2-6faa65203bf4" -Source "https://chocolatey.org/"
if ($LastExitCode -gt 0) {
  cd $origpwd
  exit 1
}
cpush "bowery.$($version).nupkg"
if ($LastExitCode -gt 0) {
  cd $origpwd
  exit 1
}

cd $origpwd
