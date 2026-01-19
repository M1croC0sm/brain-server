#!/bin/bash
set -e

# Brain Server Installation Script
# Run as: sudo ./install.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
USER="mrwolf"

echo "Installing brain-server..."

# Build if needed
if [ ! -f "$PROJECT_DIR/brain-server" ]; then
    echo "Building..."
    cd "$PROJECT_DIR"
    go build -o brain-server ./cmd/brain-server
fi

# Create vault directory structure
VAULT_PATH="/home/$USER/2ndBrain/Vault"
mkdir -p "$VAULT_PATH"/{Ideas,Projects,Financial/Ledger,Health,Life,Log,Letters/{Daily,Weekly},Research/Ideas}
chown -R "$USER:$USER" "$VAULT_PATH"

# Check for .env file
if [ ! -f "$PROJECT_DIR/.env" ]; then
    echo "ERROR: Create $PROJECT_DIR/.env from .env.example first"
    exit 1
fi

# Install systemd service
echo "Installing systemd service..."
cp "$SCRIPT_DIR/brain-server.service" /etc/systemd/system/
systemctl daemon-reload
systemctl enable brain-server

echo "Done! Start with: sudo systemctl start brain-server"
echo "View logs with: journalctl -u brain-server -f"
