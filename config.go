package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
)

type Config struct {
	DehumidifierMeasurementName string   `json:"measurement_name_dehumidifier,omitempty"`
	InfluxServer                string   `json:"influx_server"`
	InfluxBucket                string   `json:"influx_bucket"`
	InfluxUser                  string   `json:"influx_user,omitempty"`
	InfluxPass                  string   `json:"influx_password,omitempty"`
	InfluxToken                 string   `json:"influx_token,omitempty"`
	InfluxOrg                   string   `json:"influx_org,omitempty"`
	InfluxHealthCheckDisabled   bool     `json:"influx_health_check_disabled,omitempty"`
	HeartbeatURL                string   `json:"heartbeat_url,omitempty"`
	MideaArgs                   []string `json:"midea_beautiful_air_cli_discover_args"`
}

func ConfigFromFile(filename string) (Config, error) {
	f, err := os.Open(filename)
	if err != nil {
		return Config{}, fmt.Errorf("failed to open config file '%s': %w", filename, err)
	}
	defer f.Close()

	var config Config
	err = json.NewDecoder(f).Decode(&config)
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse config file '%s': %w", filename, err)
	}

	if config.DehumidifierMeasurementName == "" {
		config.DehumidifierMeasurementName = "midea_dehumidifier"
	}
	if config.InfluxServer == "" {
		return Config{}, fmt.Errorf("influx_server is required")
	}
	if config.InfluxBucket == "" {
		return Config{}, fmt.Errorf("influx_bucket is required")
	}
	if config.HeartbeatURL != "" {
		_, err = url.Parse(config.HeartbeatURL)
		if err != nil {
			return Config{}, fmt.Errorf("failed to parse heartbeat_url: %w", err)
		}
	}
	if len(config.MideaArgs) == 0 {
		return Config{}, fmt.Errorf("midea_beautiful_air_cli_discover_args is required")
	}

	return config, err
}

func DefaultCfgPath() string {
	if _, err := os.Stat("/config.json"); err == nil {
		return "/config.json"
	}
	if _, err := os.Stat("./config.json"); err == nil {
		return "./config.json"
	}
	return ""
}
