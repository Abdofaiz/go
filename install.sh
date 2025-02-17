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

# Check if port 443 is in use
if lsof -i :443 > /dev/null 2>&1; then
    print_status "Port 443 is in use. Stopping conflicting services..."
    systemctl stop nginx || true
    systemctl stop apache2 || true
    sleep 2
fi

# Install unzip if not present
apt install -y unzip

# Remove any existing Xray installation
systemctl stop xray || true
systemctl disable xray || true
rm -rf /usr/local/bin/xray /etc/xray /var/log/xray /usr/local/share/xray

# Create required directories
mkdir -p /usr/local/bin
mkdir -p /etc/xray
mkdir -p /var/log/xray

# Download and install Xray directly
cd /tmp
wget -O xray.zip https://github.com/XTLS/Xray-core/releases/download/v1.8.4/Xray-linux-64.zip
unzip -j xray.zip xray -d /usr/local/bin/
chmod +x /usr/local/bin/xray
rm -f xray.zip

# Verify xray binary works
if ! /usr/local/bin/xray version; then
    print_error "Xray binary installation failed"
    exit 1
fi

# Create minimal config with a different port initially
cat > /etc/xray/config.json << EOF
{
    "inbounds": [
        {
            "port": 10085,
            "protocol": "vmess",
            "settings": {
                "clients": []
            }
        }
    ],
    "outbounds": [
        {
            "protocol": "freedom"
        }
    ]
}
EOF

# Create service file
cat > /etc/systemd/system/xray.service << EOF
[Unit]
Description=Xray Service
After=network.target

[Service]
ExecStart=/usr/local/bin/xray run -config /etc/xray/config.json
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

# Set permissions
chmod 755 /usr/local/bin/xray
chmod 644 /etc/xray/config.json
chmod 755 /var/log/xray

# Start service
systemctl daemon-reload
systemctl enable xray
systemctl restart xray

# Verify service is running
sleep 2
if systemctl is-active --quiet xray; then
    print_success "Xray installed and running successfully!"
    
    # Now try to switch to port 443
    print_status "Configuring Xray for port 443..."
    sed -i 's/"port": 10085/"port": 443/' /etc/xray/config.json
    
    if systemctl restart xray; then
        print_success "Xray configured successfully on port 443"
    else
        print_error "Could not bind to port 443. Using alternative port 10085"
        print_status "You can change the port later in /etc/xray/config.json"
    fi
else
    print_error "Xray failed to start. Logs:"
    journalctl -u xray --no-pager -n 20
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

# Set up Go project
print_status "Setting up Go project..."
cd /root/go

# Set Go environment variables
export GO111MODULE=on
export GOPATH=""
export GOROOT=$(go env GOROOT)
export PATH=$PATH:$GOROOT/bin

# Initialize Go module
print_status "Initializing Go module..."
rm -f go.mod go.sum
go mod init vps_manager

# Create go.mod with correct version
cat > go.mod << EOF
module vps_manager

go 1.20

require (
	github.com/google/uuid v1.3.0
	golang.org/x/crypto v0.0.0-20220214200702-86341886e292
)
EOF

# Update import paths
print_status "Updating import paths..."
sed -i 's|"./protocols"|"vps_manager/protocols"|g' vps_manager.go
sed -i 's|"./config"|"vps_manager/config"|g' vps_manager.go

# Add ioutil imports for older Go versions
print_status "Updating imports for compatibility..."
find . -type f -name "*.go" -exec sed -i 's/os.ReadFile/ioutil.ReadFile/g' {} \;
find . -type f -name "*.go" -exec sed -i 's/os.WriteFile/ioutil.WriteFile/g' {} \;

# Add ioutil import to files
for file in $(find . -type f -name "*.go"); do
	if grep -q "ioutil\." "$file"; then
		sed -i '1,/^import/!b;/^import/a\\t"io/ioutil"' "$file"
	fi
done

# Download dependencies
print_status "Downloading dependencies..."
go mod download
go mod tidy

# Build the program
print_status "Building VPS Manager..."
if ! go build -o vps_manager .; then
	print_error "Failed to build. Build output:"
	go build -v -o vps_manager .
	exit 1
fi

# Install the binary
if [ -f "vps_manager" ]; then
	print_status "Installing VPS Manager binary..."
	mv vps_manager /usr/local/bin/
	chmod +x /usr/local/bin/vps_manager
else
	print_error "Build failed: binary not created"
	exit 1
fi

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