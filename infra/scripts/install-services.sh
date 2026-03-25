#!/bin/bash
# install-services.sh — Install systemd units for P2Pool Dashboard
# Usage: sudo ./install-services.sh
#
# Installs the dashboard service (auto-start on boot) and backup timer (daily).
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-/opt/p2pool-dashboard}"
SYSTEMD_DIR="/etc/systemd/system"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
UNIT_DIR="$(cd "$SCRIPT_DIR/../systemd" && pwd)"

info()  { echo "[INFO]  $*"; }
error() { echo "[ERROR] $*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || error "This script must be run as root (use sudo)"

# ---------------------------------------------------------------------------
# 1. Install units
# ---------------------------------------------------------------------------
info "Installing systemd units..."

for unit in p2pool-dashboard.service p2pool-backup.service p2pool-backup.timer; do
  cp "$UNIT_DIR/$unit" "$SYSTEMD_DIR/$unit"
  info "Installed: $SYSTEMD_DIR/$unit"
done

# ---------------------------------------------------------------------------
# 2. Reload and enable
# ---------------------------------------------------------------------------
systemctl daemon-reload

# Enable dashboard to start on boot
systemctl enable p2pool-dashboard.service
info "p2pool-dashboard.service enabled (starts on boot)"

# Enable and start backup timer
systemctl enable p2pool-backup.timer
systemctl start p2pool-backup.timer
info "p2pool-backup.timer enabled (daily at 04:00 UTC)"

# ---------------------------------------------------------------------------
# 3. Status
# ---------------------------------------------------------------------------
echo ""
echo "============================================"
echo "  Systemd units installed"
echo "============================================"
echo ""
echo "  Dashboard service:"
echo "    sudo systemctl start p2pool-dashboard"
echo "    sudo systemctl status p2pool-dashboard"
echo "    sudo systemctl stop p2pool-dashboard"
echo ""
echo "  Backup timer:"
echo "    systemctl list-timers p2pool-backup.timer"
echo "    sudo systemctl start p2pool-backup.service  (manual run)"
echo ""
echo "  Logs:"
echo "    journalctl -u p2pool-dashboard -f"
echo "    journalctl -u p2pool-backup --since today"
echo ""
