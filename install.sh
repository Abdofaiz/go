#!/bin/bash

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Function to print status
print_status() {
    echo -e "${YELLOW}[*] $1${NC}"
}

# Function to print success
print_success() {
    echo -e "${GREEN}[+] $1${NC}"
}

# Function to print error
print_error() {
    echo -e "${RED}[-] $1${NC}"
}

# Check if running as root
if [ "$(id -u)" != "0" ]; then
   print_error "This script must be run as root"
   exit 1
fi

# Update system
print_status "Updating system..."
apt update && apt upgrade -y

# Install required packages
print_status "Installing required packages..."
apt install -y \
    golang \
    nginx \
    squid \
    dropbear \
    apache2-utils \
    git \
    curl \
    wget \
    ufw \
    jq

# Install Xray
print_status "Installing Xray..."
if [ -f "/usr/local/bin/xray" ]; then
    print_status "Removing existing Xray installation..."
    systemctl stop xray || true
    systemctl disable xray || true
    rm -rf /usr/local/bin/xray
    rm -rf /etc/xray
    rm -rf /var/log/xray
    rm -f /etc/systemd/system/xray.service
fi

# Install required dependencies
apt install -y unzip daemon

# Download and install Xray binary
print_status "Downloading Xray..."
mkdir -p /tmp/xray
cd /tmp/xray
wget -q https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-64.zip
unzip -q Xray-linux-64.zip
install -m 755 xray /usr/local/bin/xray
install -m 644 geoip.dat /usr/local/share/xray/
install -m 644 geosite.dat /usr/local/share/xray/
cd -
rm -rf /tmp/xray

# Create directories and set permissions
mkdir -p /usr/local/share/xray
mkdir -p /var/log/xray
mkdir -p /etc/xray

# Create default Xray config
print_status "Configuring Xray..."
cat > /etc/xray/config.json << EOF
{
    "log": {
        "loglevel": "warning",
        "access": "/var/log/xray/access.log",
        "error": "/var/log/xray/error.log"
    },
    "inbounds": [
        {
            "port": 443,
            "protocol": "vmess",
            "settings": {
                "clients": []
            },
            "streamSettings": {
                "network": "ws",
                "wsSettings": {
                    "path": "/ws"
                }
            }
        }
    ],
    "outbounds": [
        {
            "protocol": "freedom",
            "settings": {}
        }
    ]
}
EOF

# Create Xray systemd service
print_status "Creating Xray service..."
cat > /etc/systemd/system/xray.service << EOF
[Unit]
Description=Xray Service
Documentation=https://github.com/xtls
After=network.target nss-lookup.target

[Service]
Type=simple
User=root
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
NoNewPrivileges=true
Environment=XRAY_LOCATION_ASSET=/usr/local/share/xray
ExecStart=/usr/local/bin/xray run -config /etc/xray/config.json
Restart=on-failure
RestartPreventExitStatus=23
LimitNPROC=10000
LimitNOFILE=1000000

[Install]
WantedBy=multi-user.target
EOF

# Set permissions
chown -R root:root /etc/xray
chown -R root:root /var/log/xray
chmod 644 /etc/xray/config.json
chmod 755 /var/log/xray

# Reload systemd and start Xray
print_status "Starting Xray service..."
systemctl daemon-reload
systemctl enable xray
systemctl restart xray

# Verify Xray is running
sleep 2
if systemctl is-active --quiet xray; then
    print_success "Xray installed and running successfully!"
else
    print_error "Xray installation failed. Checking logs..."
    journalctl -xe -u xray
    exit 1
fi

# Create directories
print_status "Creating directories..."
mkdir -p /etc/vps_manager
mkdir -p /var/log/vps_manager
mkdir -p /etc/xray
mkdir -p /etc/udp
mkdir -p /etc/nginx/conf.d
mkdir -p /etc/ssl/certs
mkdir -p /etc/ssl/private

# Set permissions
chmod 755 /etc/vps_manager
chmod 755 /var/log/vps_manager
chown -R root:root /etc/vps_manager

# Create project directory and download source
print_status "Setting up VPS Manager..."
WORK_DIR="/root/vps_manager"
mkdir -p $WORK_DIR
cd $WORK_DIR

# Initialize Go module
go mod init vps_manager
go get github.com/google/uuid
go get golang.org/x/crypto/bcrypt

# Create config directories
mkdir -p protocols
mkdir -p config

# Write configuration file
print_status "Creating configuration..."
cat > /etc/vps_manager/config.json << EOF
{
    "domain": "$(hostname -f)",
    "log_path": "/var/log/vps_manager/vps.log",
    "db_path": "/etc/vps_manager/users.json",
    "protocols": {
        "ssh": {
            "port": 22
        },
        "xray": {
            "port": 443,
            "config_path": "/etc/xray/config.json"
        },
        "websocket": {
            "port": 80,
            "config_path": "/etc/nginx/conf.d/websocket.conf"
        },
        "ssl": {
            "cert_path": "/etc/ssl/certs/vps.crt",
            "key_path": "/etc/ssl/private/vps.key"
        },
        "http": {
            "port": 8080,
            "config_path": "/etc/nginx/conf.d/http.conf"
        },
        "squid": {
            "port": 3128,
            "passwd_file": "/etc/squid/passwd"
        },
        "udp": {
            "port": 7300,
            "config_path": "/etc/udp/config.json"
        },
        "dropbear": {
            "port": 2222,
            "config_path": "/etc/dropbear/dropbear.conf"
        }
    }
}
EOF

# Configure firewall
print_status "Configuring firewall..."
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 8080/tcp
ufw allow 3128/tcp
ufw allow 7300/udp
ufw allow 2222/tcp
echo "y" | ufw enable

# Create systemd service
print_status "Creating systemd service..."
cat > /etc/systemd/system/vps_manager.service << EOF
[Unit]
Description=VPS Manager Service
After=network.target

[Service]
ExecStart=/usr/local/bin/vps_manager
WorkingDirectory=/etc/vps_manager
User=root
Group=root
Restart=always

[Install]
WantedBy=multi-user.target
EOF

# Build and install
print_status "Building VPS Manager..."
go build -o vps_manager
mv vps_manager /usr/local/bin/
chmod +x /usr/local/bin/vps_manager

# Start services
print_status "Starting services..."
systemctl daemon-reload
systemctl enable nginx
systemctl enable squid
systemctl enable dropbear
systemctl enable vps_manager

systemctl start nginx
systemctl start squid
systemctl start dropbear
systemctl start vps_manager

# Final status check
print_status "Checking service status..."
if systemctl is-active --quiet vps_manager; then
    print_success "VPS Manager installed and running successfully!"
    print_success "You can now use the VPS Manager"
else
    print_error "VPS Manager installation failed. Check logs for details."
fi

# Print important information
print_status "Installation completed!"
echo -e "${GREEN}Important information:${NC}"
echo "1. Configuration file: /etc/vps_manager/config.json"
echo "2. Log file: /var/log/vps_manager/vps.log"
echo "3. Database file: /etc/vps_manager/users.json"
echo "4. Service status: systemctl status vps_manager" 