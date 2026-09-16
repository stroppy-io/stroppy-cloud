#!/bin/sh
set -eu
/bin/sh /tmp/mount-data.sh
printf disk-persistence > /data/runtime-proof
/bin/sh /tmp/mount-data.sh
test "$(cat /data/runtime-proof)" = disk-persistence
findmnt --mountpoint /data
