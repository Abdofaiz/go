package protocols

import (
	"fmt"
	"os"
	"text/template"
)

// Add constants for paths and template
const (
	wsConfigPathTemplate = "/etc/nginx/conf.d/%s_websocket.conf"
	wsCertPathTemplate   = "/etc/ssl/certs/%s.crt"
	wsKeyPathTemplate    = "/etc/ssl/private/%s.key"
)

// WebSocketManager handles WebSocket proxy configuration with SSL support
type WebSocketManager struct {
	ConfigPath string
	Port       int
	CertPath   string
	KeyPath    string
}

// NewWebSocketManager creates a new WebSocket manager with the specified configuration
func NewWebSocketManager(port int, configPath string, certPath string, keyPath string) *WebSocketManager {
	return &WebSocketManager{
		ConfigPath: configPath,
		Port:       port,
		CertPath:   certPath,
		KeyPath:    keyPath,
	}
}

const websocketTemplate = `
server {
    listen {{ .Port }} ssl;
    server_name {{ .Domain }};

    ssl_certificate {{ .CertPath }};
    ssl_certificate_key {{ .KeyPath }};
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384;

    location /ws {
        proxy_pass http://127.0.0.1:10000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
`

// AddUser creates a new WebSocket configuration for the specified user
func (w *WebSocketManager) AddUser(username, domain string) error {
	tmpl, err := template.New("websocket").Parse(websocketTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %v", err)
	}

	config := struct {
		Port     int
		Domain   string
		CertPath string
		KeyPath  string
	}{
		Port:     w.Port,
		Domain:   domain,
		CertPath: fmt.Sprintf(wsCertPathTemplate, username),
		KeyPath:  fmt.Sprintf(wsKeyPathTemplate, username),
	}

	configPath := fmt.Sprintf(wsConfigPathTemplate, username)
	f, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %v", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, config); err != nil {
		os.Remove(configPath) // Cleanup on error
		return fmt.Errorf("failed to write config: %v", err)
	}

	return nil
}

// RemoveUser removes the WebSocket configuration for the specified user
func (w *WebSocketManager) RemoveUser(username string) error {
	configPath := fmt.Sprintf(wsConfigPathTemplate, username)
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove websocket config: %v", err)
	}
	return nil
}
