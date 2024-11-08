package exoscale

import (
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

type cloudConfig struct {
	Global       globalConfig
	Instances    instancesConfig
	LoadBalancer loadBalancerConfig `yaml:"loadBalancer"`
}

type globalConfig struct {
	Zone               string
	APIKey             string `yaml:"apiKey"`
	APISecret          string `yaml:"apiSecret"`
	APICredentialsFile string `yaml:"apiCredentialsFile"`
	APIEndpoint        string `yaml:"apiEndpoint"`
}

func readExoscaleConfig(config io.Reader) (cloudConfig, error) {
	cfg := cloudConfig{}

	// Unmarshall configuration file (YAML)
	if config != nil {
		err := yaml.NewDecoder(config).Decode(&cfg)
		if err != nil {
			return cloudConfig{}, err
		}
	}

	// From environment
	if value, exists := os.LookupEnv("EXOSCALE_ZONE"); exists {
		cfg.Global.Zone = value
	}
	if value, exists := os.LookupEnv("EXOSCALE_API_KEY"); exists {
		cfg.Global.APIKey = value
	}
	if value, exists := os.LookupEnv("EXOSCALE_API_SECRET"); exists {
		cfg.Global.APISecret = value
	}
	if value, exists := os.LookupEnv("EXOSCALE_API_CREDENTIALS_FILE"); exists {
		cfg.Global.APICredentialsFile = value
	}
	if value, exists := os.LookupEnv("EXOSCALE_API_ENPOINT"); exists {
		cfg.Global.APIEndpoint = value
	}

	return cfg, nil
}
