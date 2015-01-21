$desktop = $([System.Environment]::GetFolderPath([System.Environment+SpecialFolder]::DesktopDirectory))
Remove-Item "$desktop\Bowery.lnk"