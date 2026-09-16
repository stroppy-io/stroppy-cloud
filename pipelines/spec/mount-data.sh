#!/bin/sh
set -eu

# The YC recipe attaches a newly created secondary disk with device_name=data.
# Never fall back to a positional device name such as /dev/vdb.
device=/dev/disk/by-id/virtio-data
target=/data
filesystem=ext4
options=defaults
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
    actual=$(findmnt -n -o FSTYPE --target "$target")
    if [ "$actual" != "$filesystem" ]; then
        echo "existing mount has a different filesystem" >&2
        exit 1
    fi
    exit 0
fi
if [ -n "$(printf '%s' "$mounts" | tr -d '[:space:]')" ]; then
    echo "data disk is already in use" >&2
    exit 1
fi
if [ -d "$target" ] && [ -n "$(find "$target" -mindepth 1 -maxdepth 1 -print -quit)" ]; then
    echo "refusing to hide existing files beneath $target" >&2
    exit 1
fi
signatures=$(wipefs --no-act --noheadings --output TYPE "$device")
case "$signatures" in
    '')
        if ! command -v "mkfs.$filesystem" >/dev/null 2>&1; then
            case "$filesystem" in
                xfs) apt-get update; DEBIAN_FRONTEND=noninteractive apt-get install -y xfsprogs ;;
                *) echo "missing filesystem tool: mkfs.$filesystem" >&2; exit 1 ;;
            esac
        fi
        "mkfs.$filesystem" "$device"
        ;;
    "$filesystem") ;; # Retry after formatting or unmounting the same data disk.
    *) echo "data disk has unexpected signatures; refusing to format it" >&2; exit 1 ;;
esac
mkdir -p "$target"
mount -t "$filesystem" -o "$options" "$device" "$target"
uuid=$(blkid -o value -s UUID "$device")
fstype=$(blkid -o value -s TYPE "$device")
if ! awk -v target="$target" '$2 == target { found=1 } END { exit !found }' /etc/fstab; then
    printf 'UUID=%s %s %s %s 0 2\n' "$uuid" "$target" "$fstype" "$options" >> /etc/fstab
fi
findmnt --target "$target"
