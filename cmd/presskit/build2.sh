#!/bin/sh

trap cleanup 1 2 3 6

PID=""

cleanup()
{
  kill -9 $PID
  exit 1
}

echo "Running..."

$HOME/go/bin/go run .
