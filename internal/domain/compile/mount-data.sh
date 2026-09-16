#!/bin/sh
set -eu

# The YC recipe attaches a newly created secondary disk with device_name=data.
# Never fall back to a positional device name such as /dev/vdb.
device=/dev/disk/by-id/virtio-data
target=/data
attempt=0
while [ ! -b "$device" ]; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 60 ]; then
        echo "attached data disk did not appear: $device" >&2
        exit 1
    fi
    sleep 1
done
device=$(readlink -f "$device")
# A failed probe must never mean an empty disk.
mounts=$(lsblk -nr -o MOUNTPOINTS "$device")
types=$(lsblk -nr -o TYPE "$device")
if [ "$types" != disk ]; then
    echo "data device is not a whole disk without partitions" >&2
    exit 1
fi
if mountpoint -q "$target"; then
    source=$(findmnt -n -o SOURCE --target "$target")
    if [ "$(readlink -f "$source")" != "$device" ]; then
        echo "$target is mounted from a different device" >&2
        exit 1
    fi
    exit 0
fi
if [ -n "$(printf '%s' "$mounts" | tr -d '[:space:]')" ]; then
    echo "data disk is already in use" >&2
    exit 1
fi
signatures=$(wipefs --no-act --noheadings --output TYPE "$device")
case "$signatures" in
    '') mkfs.ext4 "$device" ;;
    ext4) ;; # Retry after formatting or unmounting the same data disk.
    *) echo "data disk has unexpected signatures; refusing to format it" >&2; exit 1 ;;
esac
mkdir -p "$target"
mount "$device" "$target"
uuid=$(blkid -o value -s UUID "$device")
fstype=$(blkid -o value -s TYPE "$device")
if ! awk '$2 == "/data" { found=1 } END { exit !found }' /etc/fstab; then
    printf 'UUID=%s /data %s defaults 0 2\n' "$uuid" "$fstype" >> /etc/fstab
fi
findmnt --target "$target"
