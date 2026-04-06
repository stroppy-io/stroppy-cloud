#!/usr/bin/env bash
# Opens a reverse SSH tunnel through the jump VM so that Yandex Cloud agent VMs
# can reach the local stroppy-cloud server and VictoriaMetrics.
#
# Usage:
#   ./scripts/yc-tunnel.sh                     # use defaults
#   JUMP_HOST=user@host ./scripts/yc-tunnel.sh  # override jump host
#
# Ports forwarded:
#   8080 — stroppy-cloud server (API + agent binary)
#   8428 — VictoriaMetrics (remote-write endpoint for metrics)
#
# Prerequisites:
#   - GatewayPorts yes (or clientspecified) in /etc/ssh/sshd_config on the jump VM
#   - SSH key-based access to the jump VM

set -euo pipefail

JUMP_HOST="${JUMP_HOST:-st-postgres@84.201.148.157}"
SERVER_PORT="${SERVER_PORT:-8080}"
VICTORIA_PORT="${VICTORIA_PORT:-8428}"

echo "Opening reverse tunnel through ${JUMP_HOST}"
echo "  :${SERVER_PORT}  → localhost:${SERVER_PORT}  (stroppy-cloud server)"
echo "  :${VICTORIA_PORT} → localhost:${VICTORIA_PORT} (VictoriaMetrics)"
echo ""
echo "Agents should use: STROPPY_SERVER_ADDR=http://84.201.148.157:${SERVER_PORT}"
echo "Press Ctrl+C to close the tunnel."

exec ssh -N -o ServerAliveInterval=30 -o ServerAliveCountMax=3 \
  -R "0.0.0.0:${SERVER_PORT}:localhost:${SERVER_PORT}" \
  -R "0.0.0.0:${VICTORIA_PORT}:localhost:${VICTORIA_PORT}" \
  "${JUMP_HOST}"
