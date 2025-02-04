package protocols

import (
	"fmt"
	"os"
	"text/template"
)

type UDPManager struct {
	ConfigPath string
	Port       int
}

func NewUDPManager(port int, configPath string) *UDPManager {
	return &UDPManager{
		ConfigPath: configPath,
		Port:       port,
	}
}

const udpTemplate = `{
    "listen": ":{{ .Port }}",
    "users": {
        "{{ .Username }}": "{{ .Password }}"
    },
    "timeout": 300,
    "buffer_size": 65535
}`

func (u *UDPManager) AddUser(username, password string) error {
	tmpl, err := template.New("udp").Parse(udpTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %v", err)
	}

	config := struct {
		Port     int
		Username string
		Password string
	}{
		Port:     u.Port,
		Username: username,
		Password: password,
	}

	f, err := os.Create(fmt.Sprintf("/etc/udp/%s.json", username))
	if err != nil {
		return fmt.Errorf("failed to create config file: %v", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, config); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	return nil
}

func (u *UDPManager) RemoveUser(username string) error {
	configPath := fmt.Sprintf("/etc/udp/%s.json", username)
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove UDP config: %v", err)
	}
	return nil
}
