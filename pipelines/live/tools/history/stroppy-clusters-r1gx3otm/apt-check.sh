#!/bin/sh
set -eu
apt_config=/tmp/apt.conf
if [ -n "${APT_CONFIG:-}" ]; then cat "$APT_CONFIG" > "$apt_config" || exit 1; fi;
printf '\nDPkg::Lock::Timeout "120";\n' >> "$apt_config";
export APT_CONFIG="$apt_config";

/tmp/apt-lock >/tmp/locked &
pid=$!
for i in 1 2 3 4 5; do [ -s /tmp/locked ] && break; sleep 1; done
start=$(date +%s)
apt-get -y --no-download install bash
end=$(date +%s)
wait $pid
test $((end-start)) -ge 2
echo lock-wait-check-passed
