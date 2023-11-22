package main

import (
	"encoding/json"
	"log"
	"os"
)

var Config = loadConfig()

type ConfigST struct {
	Bots   BotsConfig   `json:"bots"`
	Server ServerConfig `json:"server"`
}

type BotsConfig struct {
	Gate   string `json:"gate"`
	Garage string `json:"garage"`
}

type ServerConfig struct {
	Address   string `json:"address"`
	UseTLS    bool   `json:"useTLS"`
	TLSCert   string `json:"certificate"`
	TLSKey    string `json:"privateKey"`
	AuthToken string `json:"authToken"`
}

func loadConfig() *ConfigST {
	var config ConfigST
	data, err := os.ReadFile("config.json")
	if err == nil {
		err = json.Unmarshal(data, &config)
		if err != nil {
			log.Fatalln("Failed loading config", err)
		}
	}
	return &config
}
