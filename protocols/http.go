package protocols

import (
	"fmt"
	"os"
	"os/exec"
	"text/template"
)

type HTTPManager struct {
	ConfigPath string
	Port       int
}

func NewHTTPManager(port int, configPath string) *HTTPManager {
	return &HTTPManager{
		ConfigPath: configPath,
		Port:       port,
	}
}

const httpTemplate = `
server {
    listen {{ .Port }};
    server_name {{ .Domain }};

    location / {
        proxy_pass http://127.0.0.1:10000;
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        auth_basic "Restricted Access";
        auth_basic_user_file /etc/nginx/.htpasswd;
    }
}
`

func (h *HTTPManager) AddUser(username, password, domain string) error {
	// Create nginx config
	tmpl, err := template.New("http").Parse(httpTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %v", err)
	}

	config := struct {
		Port   int
		Domain string
	}{
		Port:   h.Port,
		Domain: domain,
	}

	f, err := os.Create(fmt.Sprintf("/etc/nginx/conf.d/%s_http.conf", username))
	if err != nil {
		return fmt.Errorf("failed to create config file: %v", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, config); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	// Add to htpasswd file
	cmd := exec.Command("htpasswd", "-b", "/etc/nginx/.htpasswd", username, password)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add to htpasswd: %v", err)
	}

	return nil
}

func (h *HTTPManager) RemoveUser(username string) error {
	// Remove nginx config
	configPath := fmt.Sprintf("/etc/nginx/conf.d/%s_http.conf", username)
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove http config: %v", err)
	}

	// Remove from htpasswd
	cmd := exec.Command("htpasswd", "-D", "/etc/nginx/.htpasswd", username)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove from htpasswd: %v", err)
	}

	return nil
}
