package config

import (
	"os"

	yaml "gopkg.in/yaml.v3"
)

type Table struct {
	Name       string   `yaml:"name"`
	Operations []string `yaml:"operations"`
}

type Tables []Table

type DBConfig struct {
	Tables   Tables `yaml:"tables"`
	Database struct {
		Type     string `yaml:"type"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Name     string `yaml:"name"`
		Os       string `yaml:"os"`
	} `yaml:"database"`
	Servers struct {
		WsPort      int    `yaml:"ws_port"`
		HTTPPort    int    `yaml:"http_port"`
		WsBaseUrl   string `yaml:"ws_base_url"`
		HttpBaseUrl string `yaml:"http_base_url"`
		IsRemote    bool   `yaml:"is_remote"`
	} `yaml:"servers"`
}

var cfg DBConfig

func ParseConfig() (DBConfig, error) {
	f, err := os.Open("realtimer.yaml")
	if err != nil {
		return DBConfig{}, err
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		return DBConfig{}, err
	}

	return cfg, nil
}
