package protocols

import (
	"fmt"
	"os/exec"
	"strings"
)

type DropbearManager struct {
	ConfigPath string
	Port       int
}

func NewDropbearManager(port int, configPath string) *DropbearManager {
	return &DropbearManager{
		ConfigPath: configPath,
		Port:       port,
	}
}

func (d *DropbearManager) AddUser(username, password string) error {
	// Create system user for Dropbear
	addUser := exec.Command("useradd", "-m", "-s", "/bin/false", username)
	if err := addUser.Run(); err != nil {
		return fmt.Errorf("failed to create dropbear user: %v", err)
	}

	// Set password
	setPass := exec.Command("chpasswd")
	setPass.Stdin = strings.NewReader(fmt.Sprintf("%s:%s", username, password))
	if err := setPass.Run(); err != nil {
		// Cleanup on failure
		exec.Command("userdel", "-r", username).Run()
		return fmt.Errorf("failed to set dropbear password: %v", err)
	}

	return nil
}

func (d *DropbearManager) RemoveUser(username string) error {
	cmd := exec.Command("userdel", "-r", username)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove dropbear user: %v", err)
	}
	return nil
}
