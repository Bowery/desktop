$ErrorActionPreference = "Stop"
$origpwd = $pwd

# Get and cd into the directory containing this script.
$cmd = $MyInvocation.MyCommand
$root = $cmd.Definition.Replace($cmd.Name, "..")
$dir = $root + "\bowery\agent"

# Build the winzip program, otherwise we'd have to use shitty vbscript.
cd "$root\scripts"
go build -o zip.exe winzip.go
if ($LastExitCode -gt 0) {
    cd $origpwd
    exit 1
}
cd $dir

# Retrieve the version.
$version = Get-Content ..\VERSION
echo "Version: $version"

# Build the agent.
#TODO: go get -u github.com/laher/goxc
if ($LastExitCode -gt 0) {
    cd $origpwd
    exit 1
}
echo "build"
#TODO: goxc -tasks-=validate "-d=$dir\pkg" "-pv=$version" -os=windows xc
if ($LastExitCode -gt 0) {
    cd $origpwd
    exit 1
}

# Zip up the binaries.
New-Item "$dir\pkg\$version\dist" -Force -ItemType directory 
ForEach ($platform in Get-ChildItem "$dir\pkg\$version") {
    $name = $platform.name
    if ($name -eq "dist") {
        continue
    }

    ..\..\scripts\zip.exe "$dir\pkg\$version\$name" "$dir\pkg\$version\dist\$($version)_$name.zip"
    if ($LastExitCode -gt 0) {
        cd $origpwd
        exit 1
    }
}

# Pack the choco pkg.
New-Item "$dir\choco\tools" -Force -ItemType directory
New-Item "$dir\choco\bowery-agent.nuspec" -Force -ItemType file -Value ""
$nuspec = (Get-Content "$dir\bowery-agent.nuspec") -replace "{{version}}","$version"
[System.IO.File]::WriteAllLines("$dir\choco\bowery-agent.nuspec", $nuspec)
$install = (Get-Content "$dir\tools\chocolateyInstall.ps1") -replace "{{version}}","$version"
[System.IO.File]::WriteAllLines("$dir\choco\tools\chocolateyInstall.ps1", $install)
cd "$dir\choco"
cpack
if ($LastExitCode -gt 0) {
    cd $origpwd
    exit 1
}

# Push the choco pkg.
cinst nuget.commandline
nuget SetApiKey "2e664545-c457-4d43-afc2-6faa65203bf4" -source http://chocolatey.org
cpush "bowery-agent.$($version).nupkg"

# loop dist and upload

cd $origpwd
