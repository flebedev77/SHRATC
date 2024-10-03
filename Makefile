buildrun:
	go build
	.\drug.daemon.exe
release:
	go build -ldflags "-s -w"