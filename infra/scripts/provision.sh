#!/bin/bash
# provision.sh — Idempotent Ubuntu VPS provisioning for P2Pool Dashboard
# Usage: sudo ./provision.sh [--deploy-user USERNAME] [--ssh-port PORT]
#
# Run on a fresh Ubuntu 22.04/24.04 LTS VPS. Safe to re-run.
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration (override via env or flags)
# ---------------------------------------------------------------------------
DEPLOY_USER="${DEPLOY_USER:-deploy}"
SSH_PORT="${SSH_PORT:-22}"
INSTALL_DIR="${INSTALL_DIR:-/opt/p2pool-dashboard}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --deploy-user) DEPLOY_USER="$2"; shift 2 ;;
    --ssh-port)    SSH_PORT="$2";    shift 2 ;;
    --install-dir) INSTALL_DIR="$2"; shift 2 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
info()  { echo "[INFO]  $*"; }
warn()  { echo "[WARN]  $*"; }
error() { echo "[ERROR] $*" >&2; exit 1; }

require_root() {
  [[ $EUID -eq 0 ]] || error "This script must be run as root (use sudo)"
}

# ---------------------------------------------------------------------------
# 1. System updates & base packages
# ---------------------------------------------------------------------------
install_base_packages() {
  info "Updating package index..."
  apt-get update -qq

  info "Installing base packages..."
  DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg \
    lsb-release \
    software-properties-common \
    ufw \
    fail2ban \
    unattended-upgrades \
    git \
    jq \
    htop \
    ncdu \
    > /dev/null

  info "Enabling unattended-upgrades..."
  dpkg-reconfigure -f noninteractive unattended-upgrades 2>/dev/null || true
}

# ---------------------------------------------------------------------------
# 2. Docker Engine + Compose plugin
# ---------------------------------------------------------------------------
install_docker() {
  if command -v docker &>/dev/null; then
    info "Docker already installed: $(docker --version)"
  else
    info "Installing Docker Engine..."

    # Add Docker official GPG key
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
      -o /etc/apt/keyrings/docker.asc
    chmod a+r /etc/apt/keyrings/docker.asc

    # Add Docker apt repository
    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
      https://download.docker.com/linux/ubuntu \
      $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
      > /etc/apt/sources.list.d/docker.list

    apt-get update -qq
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
      docker-ce \
      docker-ce-cli \
      containerd.io \
      docker-buildx-plugin \
      docker-compose-plugin \
      > /dev/null

    systemctl enable docker
    systemctl start docker
    info "Docker installed: $(docker --version)"
  fi

  # Verify compose plugin
  if docker compose version &>/dev/null; then
    info "Docker Compose plugin: $(docker compose version --short)"
  else
    error "Docker Compose plugin not found after install"
  fi
}

# ---------------------------------------------------------------------------
# 3. Deploy user
# ---------------------------------------------------------------------------
create_deploy_user() {
  if id "$DEPLOY_USER" &>/dev/null; then
    info "User '$DEPLOY_USER' already exists"
  else
    info "Creating deploy user: $DEPLOY_USER"
    useradd -m -s /bin/bash "$DEPLOY_USER"
  fi

  # Add to docker group
  usermod -aG docker "$DEPLOY_USER"
  info "User '$DEPLOY_USER' added to docker group"

  # Copy SSH authorized_keys from root if deploy user has none
  DEPLOY_HOME=$(eval echo "~$DEPLOY_USER")
  if [[ ! -f "$DEPLOY_HOME/.ssh/authorized_keys" ]]; then
    if [[ -f /root/.ssh/authorized_keys ]]; then
      info "Copying SSH keys from root to $DEPLOY_USER"
      mkdir -p "$DEPLOY_HOME/.ssh"
      cp /root/.ssh/authorized_keys "$DEPLOY_HOME/.ssh/authorized_keys"
      chown -R "$DEPLOY_USER:$DEPLOY_USER" "$DEPLOY_HOME/.ssh"
      chmod 700 "$DEPLOY_HOME/.ssh"
      chmod 600 "$DEPLOY_HOME/.ssh/authorized_keys"
    else
      warn "No SSH keys found for root — add keys to $DEPLOY_HOME/.ssh/authorized_keys manually"
    fi
  fi
}

# ---------------------------------------------------------------------------
# 4. Firewall (UFW)
# ---------------------------------------------------------------------------
configure_firewall() {
  info "Configuring UFW firewall..."

  # Reset to defaults
  ufw --force reset > /dev/null 2>&1

  ufw default deny incoming
  ufw default allow outgoing

  # SSH (custom port if changed)
  ufw allow "$SSH_PORT/tcp" comment "SSH"

  # HTTP/HTTPS for nginx
  ufw allow 80/tcp comment "HTTP"
  ufw allow 443/tcp comment "HTTPS"

  # Stratum ports for shared P2Pool nodes (TCP passthrough via nginx)
  ufw allow 3333/tcp comment "P2Pool mini stratum"
  ufw allow 3334/tcp comment "P2Pool main stratum"

  # Enable
  ufw --force enable
  info "UFW active — allowed ports: $SSH_PORT (SSH), 80 (HTTP), 443 (HTTPS), 3333 (stratum mini), 3334 (stratum main)"
}

# ---------------------------------------------------------------------------
# 5. Fail2ban
# ---------------------------------------------------------------------------
configure_fail2ban() {
  info "Configuring fail2ban..."

  cat > /etc/fail2ban/jail.local <<EOF
[DEFAULT]
bantime  = 1h
findtime = 10m
maxretry = 5

[sshd]
enabled  = true
port     = $SSH_PORT
filter   = sshd
logpath  = /var/log/auth.log
maxretry = 3
bantime  = 24h
EOF

  systemctl enable fail2ban
  systemctl restart fail2ban
  info "fail2ban enabled — SSH: 3 attempts, 24h ban"
}

# ---------------------------------------------------------------------------
# 6. Kernel tuning
# ---------------------------------------------------------------------------
tune_kernel() {
  info "Applying kernel tuning..."

  SYSCTL_FILE="/etc/sysctl.d/99-p2pool-dashboard.conf"
  cat > "$SYSCTL_FILE" <<EOF
# P2Pool Dashboard — kernel tuning
vm.swappiness = 10
fs.file-max = 65535
net.core.somaxconn = 1024
net.ipv4.tcp_tw_reuse = 1
net.ipv4.ip_local_port_range = 1024 65535
EOF

  sysctl --system > /dev/null 2>&1
  info "Kernel parameters applied"
}

# ---------------------------------------------------------------------------
# 7. Directory structure
# ---------------------------------------------------------------------------
create_directories() {
  info "Creating directory structure at $INSTALL_DIR..."

  mkdir -p "$INSTALL_DIR"/{secrets,certs,backups}

  # Set ownership
  chown -R "$DEPLOY_USER:$DEPLOY_USER" "$INSTALL_DIR"
  chmod 700 "$INSTALL_DIR/secrets"
  chmod 755 "$INSTALL_DIR/certs"
  chmod 755 "$INSTALL_DIR/backups"

  info "Directory structure ready:"
  info "  $INSTALL_DIR/secrets/  (mode 700)"
  info "  $INSTALL_DIR/certs/    (mode 755)"
  info "  $INSTALL_DIR/backups/  (mode 755)"
}

# ---------------------------------------------------------------------------
# 8. Docker daemon configuration
# ---------------------------------------------------------------------------
configure_docker_daemon() {
  info "Configuring Docker daemon..."

  mkdir -p /etc/docker
  cat > /etc/docker/daemon.json <<EOF
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  },
  "storage-driver": "overlay2",
  "live-restore": true
}
EOF

  systemctl restart docker
  info "Docker daemon configured (log rotation: 10MB x 3 files, live-restore enabled)"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  require_root

  echo "============================================"
  echo "  P2Pool Dashboard — VPS Provisioning"
  echo "============================================"
  echo ""
  echo "  Deploy user:  $DEPLOY_USER"
  echo "  SSH port:     $SSH_PORT"
  echo "  Install dir:  $INSTALL_DIR"
  echo ""

  install_base_packages
  install_docker
  create_deploy_user
  configure_firewall
  configure_fail2ban
  tune_kernel
  create_directories
  configure_docker_daemon

  echo ""
  echo "============================================"
  echo "  Provisioning complete!"
  echo "============================================"
  echo ""
  echo "  Next steps:"
  echo "  1. Run generate-secrets.sh to create secrets"
  echo "  2. Clone the repo into $INSTALL_DIR"
  echo "  3. Run setup-tls.sh for certificates"
  echo "  4. Run deploy.sh to start the stack"
  echo ""
  echo "  SSH into the VPS as '$DEPLOY_USER' going forward."
  echo "  Root login should be disabled after verifying access."
  echo ""
}

main "$@"
