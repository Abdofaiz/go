package protocols

import (
	"fmt"
	"os/exec"
)

type SquidManager struct {
	PasswdFile string
	Port       int
}

func NewSquidManager(port int, passwdFile string) *SquidManager {
	return &SquidManager{
		PasswdFile: passwdFile,
		Port:       port,
	}
}

func (s *SquidManager) AddUser(username, password string) error {
	// Create htpasswd entry
	cmd := exec.Command("htpasswd", "-b", s.PasswdFile, username, password)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add squid user: %v", err)
	}

	return nil
}

func (s *SquidManager) RemoveUser(username string) error {
	// Remove from htpasswd file
	cmd := exec.Command("htpasswd", "-D", s.PasswdFile, username)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove squid user: %v", err)
	}

	return nil
}
