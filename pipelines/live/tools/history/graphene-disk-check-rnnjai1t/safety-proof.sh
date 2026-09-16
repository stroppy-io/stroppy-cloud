#!/bin/sh
set -eu
umount /data
mock=$(mktemp -d)
trap 'rm -rf "$mock"' EXIT
printf '#!/bin/sh\nexit 73\n' > "$mock/wipefs"
printf '#!/bin/sh\ntouch /tmp/unexpected-format\nexit 74\n' > "$mock/mkfs.ext4"
chmod +x "$mock/wipefs" "$mock/mkfs.ext4"
if PATH="$mock:$PATH" /bin/sh /tmp/mount-data.sh; then
  echo 'FAIL: ignored probe failure'; exit 1
fi
test ! -e /tmp/unexpected-format
# Root disk includes partitions and must be rejected before formatting.
sed 's|device=/dev/disk/by-id/virtio-data|device=/dev/vda|' /tmp/mount-data.sh > "$mock/root-check.sh"
if PATH="$mock:$PATH" /bin/sh "$mock/root-check.sh"; then
  echo 'FAIL: accepted root disk'; exit 1
fi
test ! -e /tmp/unexpected-format
/bin/sh /tmp/mount-data.sh
test "$(cat /data/runtime-proof)" = disk-persistence
printf 'PASS: probe errors and root disk are rejected; existing data survives\n'
