#!/bin/sh

trap cleanup 1 2 3 6

PID=""

cleanup()
{
  kill -9 $PID
  exit 1
}

/home/jmacd/go/bin/go build .

echo "Ready to run..."
sleep 3600
