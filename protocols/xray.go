package protocols

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"

	"github.com/google/uuid"
)

// Add constants for configuration
const (
	xrayDefaultPort = 443
	xrayConfigPath  = "/etc/xray/config.json"
)

// XrayConfig represents the Xray server configuration structure
type XrayConfig struct {
	Inbounds []struct {
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
		Settings struct {
			Clients []struct {
				ID    string `json:"id"`
				Email string `json:"email"`
			} `json:"clients"`
		} `json:"settings"`
	} `json:"inbounds"`
}

// XrayManager handles Xray server configuration and user management
type XrayManager struct {
	ConfigPath string
	Port       int
}

// NewXrayManager creates a new Xray manager with the specified configuration
func NewXrayManager(port int, configPath string) *XrayManager {
	return &XrayManager{
		Port:       port,
		ConfigPath: configPath,
	}
}

// loadConfig reads and parses the Xray configuration file
func (x *XrayManager) loadConfig() (*XrayConfig, error) {
	data, err := ioutil.ReadFile(x.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %v", err)
	}

	var config XrayConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}

	return &config, nil
}

// saveConfig writes the Xray configuration to file
func (x *XrayManager) saveConfig(config *XrayConfig) error {
	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	if err := ioutil.WriteFile(x.ConfigPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	return nil
}

// generateUUID creates a new random UUID for user identification
func generateUUID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID: %v", err)
	}
	return id.String(), nil
}

// AddUser adds a new user to the Xray configuration
func (x *XrayManager) AddUser(username string) error {
	config, err := x.loadConfig()
	if err != nil {
		return err
	}

	// Generate UUID for user
	uuid, err := generateUUID()
	if err != nil {
		return err
	}

	// Add user to each compatible inbound
	for i, inbound := range config.Inbounds {
		if inbound.Protocol == "vmess" || inbound.Protocol == "vless" {
			config.Inbounds[i].Settings.Clients = append(
				config.Inbounds[i].Settings.Clients,
				struct {
					ID    string `json:"id"`
					Email string `json:"email"`
				}{
					ID:    uuid,
					Email: username,
				},
			)
		}
	}

	if err := x.saveConfig(config); err != nil {
		return err
	}

	// Restart Xray service
	cmd := exec.Command("systemctl", "restart", "xray")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restart xray service: %v", err)
	}

	return nil
}

// RemoveUser removes a user from the Xray configuration
func (x *XrayManager) RemoveUser(username string) error {
	config, err := x.loadConfig()
	if err != nil {
		return err
	}

	// Remove user from all inbounds
	for i, inbound := range config.Inbounds {
		if inbound.Protocol == "vmess" || inbound.Protocol == "vless" {
			newClients := make([]struct {
				ID    string `json:"id"`
				Email string `json:"email"`
			}, 0)

			for _, client := range inbound.Settings.Clients {
				if client.Email != username {
					newClients = append(newClients, client)
				}
			}

			config.Inbounds[i].Settings.Clients = newClients
		}
	}

	return x.saveConfig(config)
}
