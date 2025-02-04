package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	Domain    string         `json:"domain"`
	LogPath   string         `json:"log_path"`
	DbPath    string         `json:"db_path"`
	Protocols ProtocolConfig `json:"protocols"`
}

type ProtocolConfig struct {
	SSH struct {
		Port int `json:"port"`
	} `json:"ssh"`
	Xray struct {
		Port       int    `json:"port"`
		ConfigPath string `json:"config_path"`
	} `json:"xray"`
	WebSocket struct {
		Port       int    `json:"port"`
		ConfigPath string `json:"config_path"`
	} `json:"websocket"`
	SSL struct {
		CertPath string `json:"cert_path"`
		KeyPath  string `json:"key_path"`
	} `json:"ssl"`
	HTTP struct {
		Port       int    `json:"port"`
		ConfigPath string `json:"config_path"`
	} `json:"http"`
	Squid struct {
		Port       int    `json:"port"`
		PasswdFile string `json:"passwd_file"`
	} `json:"squid"`
	UDP struct {
		Port       int    `json:"port"`
		ConfigPath string `json:"config_path"`
	} `json:"udp"`
	Dropbear struct {
		Port       int    `json:"port"`
		ConfigPath string `json:"config_path"`
	} `json:"dropbear"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
