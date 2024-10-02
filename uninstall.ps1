Write-Output "Uninstaller for drug.daemon.exe"
Write-Output "Uninstalling..."

$malwareName = "grg2"
$appDataPath = $env:APPDATA

# Kill it if its running
Write-Output "Killing..."
taskkill.exe /F /IM drug.daemon.exe
sc.exe stop $malwareName
# Delete executable
Write-Output "Deleting..."
Remove-Item -Recurse -Path "$appDataPath\$malwareName" 2> $null
# Delete startup key
Write-Output "Forgetting..."
Remove-Item -Path "HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\Run\$malwareName" 2> $null
# Remove the startup tasksch
Unregister-ScheduledTask -TaskName "$malwareName" -Confirm:$false 2> $null
# Remove service
sc.exe delete $malwareName

Write-Output "Finished"