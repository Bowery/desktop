Write-Host "Stopping and removing Windows Service 'Bowery Agent'."
net stop "Bowery Agent" | Out-Null
nssm remove "Bowery Agent" confirm | Out-Null
