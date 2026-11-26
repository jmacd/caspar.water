all: edgemon.bbb edgemon.local

edgemon.bbb: main.go ../../go.mod ../../go.sum
	GOOS=linux GOARCH=arm GOARM=7 go build -o edgemon.bbb .

edgemon.local: main.go ../../go.mod ../../go.sum
	go build -o edgemon.local .
