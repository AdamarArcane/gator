package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

func Read() (Config, error) {
	ConfigFilePath, err := getConfigFilePath()
	if err != nil {
		return Config{}, fmt.Errorf("failed to get config file path: %w", err)
	}

	fullFilePath := ConfigFilePath + configFileName

	fileContent, err := os.ReadFile(fullFilePath)
	if err != nil {
		return Config{}, fmt.Errorf("error reading file: %w", err)
	}

	var config Config
	err = json.Unmarshal(fileContent, &config)
	if err != nil {
		log.Printf("Error decoding config: %s", err)
		return Config{}, fmt.Errorf("error decoding JSON: %w", err)
	}

	return config, nil
}

type Config struct {
	Db_url            string `json:"db_url"`
	Current_user_name string `json:"current_user_name"`
}

func (cfg *Config) SetUser(name string) error {
	ConfigFilePath, err := getConfigFilePath()
	if err != nil {
		return fmt.Errorf("failed to get config file path: %w", err)
	}

	fullFilePath := ConfigFilePath + configFileName

	fileContent, err := os.ReadFile(fullFilePath)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	var config Config
	err = json.Unmarshal(fileContent, &config)
	if err != nil {
		log.Printf("Error decoding config: %s", err)
		return fmt.Errorf("error decoding JSON: %w", err)
	}

	config.Current_user_name = name

	updatedContent, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("error encoding JSON: %w", err)
	}

	err = os.WriteFile(fullFilePath, updatedContent, 0644)
	if err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	*cfg = config

	return nil
}

// ===== Helper Functions =====

const configFileName = ".gatorconfig.json"

func getConfigFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("error finding user home directory: %w", err)
	}
	ConfigFilePath := homeDir + "/"
	return ConfigFilePath, nil
}
