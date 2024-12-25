buildrun:
	go build
	.\trojan.client.exe
release:
	go build -ldflags "-s -w"
