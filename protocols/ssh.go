package protocols

import (
	"fmt"
	"os/exec"
	"strings"
)

type SSHManager struct {
	Port int
}

func NewSSHManager(port int) *SSHManager {
	return &SSHManager{
		Port: port,
	}
}

func (s *SSHManager) AddUser(username, password string) error {
	// Create system user
	cmd := exec.Command("useradd", "-m", "-s", "/bin/false", username)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create system user: %v", err)
	}

	// Set password
	setPasswd := exec.Command("chpasswd")
	setPasswd.Stdin = strings.NewReader(fmt.Sprintf("%s:%s", username, password))
	if err := setPasswd.Run(); err != nil {
		return fmt.Errorf("failed to set password: %v", err)
	}

	return nil
}

func (s *SSHManager) RemoveUser(username string) error {
	cmd := exec.Command("userdel", "-r", username)
	return cmd.Run()
}
