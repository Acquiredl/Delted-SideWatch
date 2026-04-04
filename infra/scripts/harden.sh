#!/bin/bash
# harden.sh — Post-provision security hardening for P2Pool Dashboard VPS
# Usage: sudo ./harden.sh [--ssh-port PORT] [--disable-root]
#
# Run AFTER provision.sh and AFTER verifying you can SSH as the deploy user.
# Safe to re-run.
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
SSH_PORT="${SSH_PORT:-22}"
DISABLE_ROOT=false
DEPLOY_USER="${DEPLOY_USER:-deploy}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --ssh-port)     SSH_PORT="$2";    shift 2 ;;
    --disable-root) DISABLE_ROOT=true; shift ;;
    --deploy-user)  DEPLOY_USER="$2"; shift 2 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
info()  { echo "[INFO]  $*"; }
warn()  { echo "[WARN]  $*"; }
error() { echo "[ERROR] $*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || error "This script must be run as root (use sudo)"

# ---------------------------------------------------------------------------
# 1. SSH hardening
# ---------------------------------------------------------------------------
harden_ssh() {
  info "Hardening SSH configuration..."

  local sshd_config="/etc/ssh/sshd_config"
  local hardened="/etc/ssh/sshd_config.d/99-p2pool-hardening.conf"

  # Use a drop-in config to avoid breaking the main sshd_config
  mkdir -p /etc/ssh/sshd_config.d

  cat > "$hardened" <<EOF
# P2Pool Dashboard — SSH hardening
Port $SSH_PORT
PasswordAuthentication no
PubkeyAuthentication yes
PermitEmptyPasswords no
ChallengeResponseAuthentication no
UsePAM yes
X11Forwarding no
MaxAuthTries 3
LoginGraceTime 30
ClientAliveInterval 300
ClientAliveCountMax 2
EOF

  if [[ "$DISABLE_ROOT" == true ]]; then
    echo "PermitRootLogin no" >> "$hardened"
    info "Root SSH login DISABLED"
  else
    echo "PermitRootLogin prohibit-password" >> "$hardened"
    info "Root SSH login restricted to key-only"
  fi

  # Ensure the main config includes drop-ins
  if ! grep -q "Include /etc/ssh/sshd_config.d/" "$sshd_config" 2>/dev/null; then
    echo "Include /etc/ssh/sshd_config.d/*.conf" >> "$sshd_config"
  fi

  # Validate config before restarting
  if sshd -t 2>/dev/null; then
    systemctl restart sshd
    info "SSH hardened and restarted (port $SSH_PORT, key-only auth)"
  else
    rm -f "$hardened"
    error "SSH config validation failed — reverted changes"
  fi
}

# ---------------------------------------------------------------------------
# 2. Docker resource limits in daemon
# ---------------------------------------------------------------------------
configure_docker_limits() {
  info "Configuring Docker daemon limits..."

  # Read existing daemon.json and merge
  local daemon_json="/etc/docker/daemon.json"
  local tmp_json="/tmp/daemon.json.tmp"

  if [[ -f "$daemon_json" ]]; then
    # Merge with existing config
    jq '. + {
      "log-driver": "json-file",
      "log-opts": {
        "max-size": "10m",
        "max-file": "3"
      },
      "storage-driver": "overlay2",
      "live-restore": true,
      "default-ulimits": {
        "nofile": {
          "Name": "nofile",
          "Hard": 65535,
          "Soft": 65535
        }
      },
      "no-new-privileges": true
    }' "$daemon_json" > "$tmp_json" 2>/dev/null || {
      # If jq fails (malformed json), write fresh
      warn "Existing daemon.json could not be parsed — writing fresh config"
      cat > "$tmp_json" <<'DJSON'
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  },
  "storage-driver": "overlay2",
  "live-restore": true,
  "default-ulimits": {
    "nofile": {
      "Name": "nofile",
      "Hard": 65535,
      "Soft": 65535
    }
  },
  "no-new-privileges": true
}
DJSON
    }
  else
    cat > "$tmp_json" <<'DJSON'
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  },
  "storage-driver": "overlay2",
  "live-restore": true,
  "default-ulimits": {
    "nofile": {
      "Name": "nofile",
      "Hard": 65535,
      "Soft": 65535
    }
  },
  "no-new-privileges": true
}
DJSON
  fi

  mv "$tmp_json" "$daemon_json"
  systemctl restart docker
  info "Docker daemon: log rotation (10MB x 3), no-new-privileges, ulimits configured"
}

# ---------------------------------------------------------------------------
# 3. Verify production resource limits overlay exists
# ---------------------------------------------------------------------------
verify_resource_overrides() {
  local install_dir="${INSTALL_DIR:-/opt/p2pool-dashboard}"
  local override_file="$install_dir/docker-compose.prod.yml"

  if [[ -f "$override_file" ]]; then
    info "Production overlay found: $override_file (managed via git)"
  else
    warn "Production overlay missing: $override_file"
    warn "Run 'git pull' to get the checked-in docker-compose.prod.yml"
  fi
}

# ---------------------------------------------------------------------------
# 4. Shared memory and tmp hardening
# ---------------------------------------------------------------------------
harden_filesystem() {
  info "Hardening filesystem mounts..."

  # Secure shared memory
  if ! grep -q '/run/shm' /etc/fstab 2>/dev/null; then
    echo "tmpfs /run/shm tmpfs defaults,noexec,nosuid 0 0" >> /etc/fstab
    info "Shared memory hardened (noexec, nosuid)"
  fi
}

# ---------------------------------------------------------------------------
# 5. Automatic security updates
# ---------------------------------------------------------------------------
configure_auto_updates() {
  info "Configuring automatic security updates..."

  cat > /etc/apt/apt.conf.d/50unattended-upgrades <<'EOF'
Unattended-Upgrade::Allowed-Origins {
    "${distro_id}:${distro_codename}-security";
    "${distro_id}ESMApps:${distro_codename}-apps-security";
    "${distro_id}ESM:${distro_codename}-infra-security";
};

Unattended-Upgrade::AutoFixInterruptedDpkg "true";
Unattended-Upgrade::Remove-Unused-Kernel-Packages "true";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
Unattended-Upgrade::Automatic-Reboot "false";
EOF

  cat > /etc/apt/apt.conf.d/20auto-upgrades <<'EOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::AutocleanInterval "7";
EOF

  info "Auto security updates enabled (no auto-reboot)"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  echo "============================================"
  echo "  P2Pool Dashboard — Security Hardening"
  echo "============================================"
  echo ""

  harden_ssh
  configure_docker_limits
  verify_resource_overrides
  harden_filesystem
  configure_auto_updates

  echo ""
  echo "============================================"
  echo "  Hardening complete!"
  echo "============================================"
  echo ""
  echo "  Applied:"
  echo "    - SSH: key-only auth, port $SSH_PORT, max 3 attempts"
  echo "    - Docker: log rotation, no-new-privileges, ulimits"
  echo "    - Containers: memory/CPU limits via docker-compose.prod.yml (git-managed)"
  echo "    - Filesystem: shared memory hardened"
  echo "    - Updates: automatic security patches (no auto-reboot)"
  echo ""
  echo "  IMPORTANT: Verify you can still SSH in before closing"
  echo "  this session! Test from another terminal:"
  echo ""
  echo "    ssh -p $SSH_PORT $DEPLOY_USER@<vps-ip>"
  echo ""
}

main "$@"
