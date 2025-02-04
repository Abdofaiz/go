package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"./config"
	"./protocols"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Username   string    `json:"username"`
	Password   string    `json:"password"`
	ExpireDate time.Time `json:"expire_date"`
	Protocols  []string  `json:"protocols"`
}

type VPSManager struct {
	Users        []User
	Config       *config.Config
	SSHMgr       *protocols.SSHManager
	XrayMgr      *protocols.XrayManager
	WebSocketMgr *protocols.WebSocketManager
	SSLMgr       *protocols.SSLManager
	HTTPMgr      *protocols.HTTPManager
	SquidMgr     *protocols.SquidManager
	UDPMgr       *protocols.UDPManager
	DropbearMgr  *protocols.DropbearManager
	LogFile      *os.File
}

func NewVPSManager(configPath string) (*VPSManager, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %v", err)
	}

	logFile, err := os.OpenFile(cfg.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	return &VPSManager{
		Users:   make([]User, 0),
		Config:  cfg,
		SSHMgr:  protocols.NewSSHManager(cfg.Protocols.SSH.Port),
		XrayMgr: protocols.NewXrayManager(cfg.Protocols.Xray.Port, cfg.Protocols.Xray.ConfigPath),
		WebSocketMgr: protocols.NewWebSocketManager(
			cfg.Protocols.WebSocket.Port,
			cfg.Protocols.WebSocket.ConfigPath,
			cfg.Protocols.SSL.CertPath,
			cfg.Protocols.SSL.KeyPath,
		),
		SSLMgr:      protocols.NewSSLManager(cfg.Protocols.SSL.CertPath, cfg.Protocols.SSL.KeyPath),
		HTTPMgr:     protocols.NewHTTPManager(cfg.Protocols.HTTP.Port, cfg.Protocols.HTTP.ConfigPath),
		SquidMgr:    protocols.NewSquidManager(cfg.Protocols.Squid.Port, cfg.Protocols.Squid.PasswdFile),
		UDPMgr:      protocols.NewUDPManager(cfg.Protocols.UDP.Port, cfg.Protocols.UDP.ConfigPath),
		DropbearMgr: protocols.NewDropbearManager(cfg.Protocols.Dropbear.Port, cfg.Protocols.Dropbear.ConfigPath),
		LogFile:     logFile,
	}, nil
}

func (vm *VPSManager) AddUser(username, password string, expireDays int) error {
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	// Add system user
	if err := vm.SSHMgr.AddUser(username, password); err != nil {
		return err
	}

	// Add to Xray
	if err := vm.XrayMgr.AddUser(username); err != nil {
		vm.SSHMgr.RemoveUser(username)
		return err
	}

	// Setup WebSocket
	domain := fmt.Sprintf("%s.%s", username, vm.Config.Domain)
	if err := vm.WebSocketMgr.AddUser(username, domain); err != nil {
		vm.SSHMgr.RemoveUser(username)
		vm.XrayMgr.RemoveUser(username)
		return err
	}

	// Generate SSL certificate
	if err := vm.SSLMgr.GenerateCertificate(domain); err != nil {
		vm.SSHMgr.RemoveUser(username)
		vm.XrayMgr.RemoveUser(username)
		return err
	}

	// Add HTTP proxy
	if err := vm.HTTPMgr.AddUser(username, password, domain); err != nil {
		vm.cleanup(username)
		return err
	}

	// Add Squid proxy
	if err := vm.SquidMgr.AddUser(username, password); err != nil {
		vm.cleanup(username)
		return err
	}

	// Add UDP configuration
	if err := vm.UDPMgr.AddUser(username, password); err != nil {
		vm.cleanup(username)
		return err
	}

	// Add Dropbear user
	if err := vm.DropbearMgr.AddUser(username, password); err != nil {
		vm.cleanup(username)
		return err
	}

	expireDate := time.Now().AddDate(0, 0, expireDays)
	newUser := User{
		Username:   username,
		Password:   string(hashedPassword),
		ExpireDate: expireDate,
		Protocols:  []string{"ssh", "ssl", "websocket", "http", "squid", "xray", "udp", "dropbear"},
	}

	vm.Users = append(vm.Users, newUser)
	vm.logAction("AddUser", fmt.Sprintf("Added user %s with expiration %v", username, expireDate))
	return vm.saveToFile()
}

func (vm *VPSManager) RemoveUser(username string) error {
	// Find user first
	var found bool
	for i, user := range vm.Users {
		if user.Username == username {
			vm.Users = append(vm.Users[:i], vm.Users[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("user not found")
	}

	// Remove from all protocols
	var errors []string

	// Remove SSH user
	if err := vm.SSHMgr.RemoveUser(username); err != nil {
		errors = append(errors, fmt.Sprintf("SSH: %v", err))
	}

	// Remove from Xray
	if err := vm.XrayMgr.RemoveUser(username); err != nil {
		errors = append(errors, fmt.Sprintf("Xray: %v", err))
	}

	// Remove WebSocket configuration
	if err := vm.WebSocketMgr.RemoveUser(username); err != nil {
		errors = append(errors, fmt.Sprintf("WebSocket: %v", err))
	}

	// Remove SSL certificates
	if err := vm.SSLMgr.RemoveUser(username); err != nil {
		errors = append(errors, fmt.Sprintf("SSL: %v", err))
	}

	// Add HTTP and Squid to removal process
	if err := vm.HTTPMgr.RemoveUser(username); err != nil {
		errors = append(errors, fmt.Sprintf("HTTP: %v", err))
	}

	if err := vm.SquidMgr.RemoveUser(username); err != nil {
		errors = append(errors, fmt.Sprintf("Squid: %v", err))
	}

	// Remove UDP configuration
	if err := vm.UDPMgr.RemoveUser(username); err != nil {
		errors = append(errors, fmt.Sprintf("UDP: %v", err))
	}

	// Remove Dropbear user
	if err := vm.DropbearMgr.RemoveUser(username); err != nil {
		errors = append(errors, fmt.Sprintf("Dropbear: %v", err))
	}

	// Save changes to file
	if err := vm.saveToFile(); err != nil {
		errors = append(errors, fmt.Sprintf("Save to file: %v", err))
	}

	vm.logAction("RemoveUser", fmt.Sprintf("Removed user %s", username))

	// If there were any errors, return them all
	if len(errors) > 0 {
		return fmt.Errorf("errors removing user: %v", errors)
	}

	return nil
}

func (vm *VPSManager) ListUsers() {
	fmt.Println("Current Users:")
	fmt.Printf("%-15s %-25s %-30s\n", "Username", "Expire Date", "Protocols")
	fmt.Println("--------------------------------------------------------")

	for _, user := range vm.Users {
		fmt.Printf("%-15s %-25s %-30v\n",
			user.Username,
			user.ExpireDate.Format("2006-01-02"),
			user.Protocols)
	}
}

func (vm *VPSManager) CheckExpiredUsers() {
	now := time.Now()
	expired := make([]string, 0)

	for _, user := range vm.Users {
		if now.After(user.ExpireDate) {
			expired = append(expired, user.Username)
		}
	}

	for _, username := range expired {
		fmt.Printf("Removing expired user: %s\n", username)
		vm.RemoveUser(username)
	}
}

func (vm *VPSManager) saveToFile() error {
	data, err := json.MarshalIndent(vm.Users, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(vm.Config.DbPath, data, 0644)
}

func (vm *VPSManager) loadFromFile() error {
	data, err := os.ReadFile(vm.Config.DbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &vm.Users)
}

func (vm *VPSManager) logAction(action, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] %s: %s\n", timestamp, action, message)
	vm.LogFile.WriteString(logEntry)
}

func (vm *VPSManager) cleanup(username string) {
	vm.SSHMgr.RemoveUser(username)
	vm.XrayMgr.RemoveUser(username)
	vm.WebSocketMgr.RemoveUser(username)
	vm.SSLMgr.RemoveUser(username)
	vm.HTTPMgr.RemoveUser(username)
	vm.SquidMgr.RemoveUser(username)
	vm.UDPMgr.RemoveUser(username)
	vm.DropbearMgr.RemoveUser(username)
}

func main() {
	manager, err := NewVPSManager("config.json")
	if err != nil {
		log.Fatalf("Failed to initialize VPS manager: %v", err)
	}
	if err := manager.loadFromFile(); err != nil {
		fmt.Printf("Error loading users: %v\n", err)
		return
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\n=== VPS Management System ===")
		fmt.Println("1. Add User")
		fmt.Println("2. Remove User")
		fmt.Println("3. List Users")
		fmt.Println("4. Check Expired Users")
		fmt.Println("5. Exit")
		fmt.Print("Choose an option: ")

		var choice int
		fmt.Scanf("%d", &choice)

		switch choice {
		case 1:
			fmt.Print("Enter username: ")
			username, _ := reader.ReadString('\n')
			username = username[:len(username)-1]

			fmt.Print("Enter password: ")
			password, _ := reader.ReadString('\n')
			password = password[:len(password)-1]

			fmt.Print("Enter expiration days: ")
			var days int
			fmt.Scanf("%d", &days)

			if err := manager.AddUser(username, password, days); err != nil {
				fmt.Printf("Error adding user: %v\n", err)
			} else {
				fmt.Println("User added successfully")
			}

		case 2:
			fmt.Print("Enter username to remove: ")
			username, _ := reader.ReadString('\n')
			username = username[:len(username)-1]

			if err := manager.RemoveUser(username); err != nil {
				fmt.Printf("Error removing user: %v\n", err)
			} else {
				fmt.Println("User removed successfully")
			}

		case 3:
			manager.ListUsers()

		case 4:
			manager.CheckExpiredUsers()

		case 5:
			fmt.Println("Goodbye!")
			return

		default:
			fmt.Println("Invalid option")
		}
	}
}
