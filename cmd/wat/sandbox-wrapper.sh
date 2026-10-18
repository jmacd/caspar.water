#!/bin/sh
set +e

export WAT_STATEMENT_PYTHON=/usr/local/bin/python3.13
/usr/local/bin/wat "$@"
status=$?

find accounting -type f -name '.wat-backup-*' -delete
exit "${status}"
