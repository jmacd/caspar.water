all: edgemon.bbb edgemon.local

edgemon.bbb: main.go
	GOOS=linux GOARCH=arm GOARM=7 go build -o edgemon.bbb .

edgemon.local: main.go
	go build -o edgemon.local .
