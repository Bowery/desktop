@echo off

set install=%ProgramFiles%\Bowery

title Bowery Installer
if not exist "%install%" mkdir "%install%"
xcopy /E /Y * "%install%"
echo."%PATH%" | find "%install%" > NUL || (setx PATH "%PATH%;%install%")
"%install%\rmmanifest.exe" "%install%\bowery.exe"

echo Set shell = Wscript.CreateObject("Wscript.Shell") > shortcut.vbs
echo Set link = shell.CreateShortcut("%USERPROFILE%\Desktop\Bowery.lnk") >> shortcut.vbs
echo link.Description = "Bowery Desktop" >> shortcut.vbs
echo link.TargetPath = "%install%\bowery.exe" >> shortcut.vbs
echo link.IconLocation = "%install%\logo.ico, 0" >> shortcut.vbs
echo link.Save >> shortcut.vbs
cscript shortcut.vbs
