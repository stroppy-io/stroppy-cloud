package provision

import (
	"encoding/base64"
	"fmt"
)

// bootPrefix quiets a benchmark machine before anything else runs on it:
// Ubuntu's unattended upgrades start at a random time after boot, and
// needrestart then restarts systemd-networkd, resolved and journald in the
// middle of a deploy or a measurement; a run machine is thrown away
// afterwards, so it needs no updates of its own.
//
// doc: https://documentation.ubuntu.com/server/how-to/software/automatic-updates/
// doc: https://manpages.ubuntu.com/manpages/noble/man1/needrestart.1.html — $nrconf{restart}
const bootPrefix = `systemctl stop unattended-upgrades.service apt-daily.timer apt-daily-upgrade.timer apt-daily.service apt-daily-upgrade.service 2>/dev/null || true
systemctl mask unattended-upgrades.service apt-daily.timer apt-daily-upgrade.timer apt-daily.service apt-daily-upgrade.service 2>/dev/null || true
if [ -d /etc/needrestart ]; then
  mkdir -p /etc/needrestart/conf.d
  echo "\$nrconf{restart} = 'l';" > /etc/needrestart/conf.d/90-stroppy.conf
fi
`

// bootScript is a machine's user-data: the quiet-down prefix, then the
// agent's own install script run with its own interpreter.
func bootScript(agentScript string) string {
	if agentScript == "" {
		return ""
	}
	return fmt.Sprintf("#!/bin/sh\n%secho %s | base64 -d > /run/stroppy-agent-install\nchmod 0700 /run/stroppy-agent-install\nexec /run/stroppy-agent-install\n",
		bootPrefix, base64.StdEncoding.EncodeToString([]byte(agentScript)))
}
