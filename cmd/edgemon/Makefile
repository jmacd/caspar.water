edgemon: main.go
	GOOS=linux GOARCH=arm GOARM=7 go build -ldflags "-w -s" -o edgemon .
	scp edgemon shell@upperwell.local:edgemon.new
	ssh shell@upperwell.local mv edgemon.new edgemon

# Need to run:
#   sudo /usr/sbin/setcap cap_net_raw=+ep ./edgemon
