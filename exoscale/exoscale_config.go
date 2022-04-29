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
	ApiKey             string `yaml:"apiKey"`
	ApiSecret          string `yaml:"apiSecret"`
	ApiCredentialsFile string `yaml:"apiCredentialsFile"`
	ApiEnvironment     string `yaml:"apiEnvironment"`
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
		cfg.Global.ApiKey = value
	}
	if value, exists := os.LookupEnv("EXOSCALE_API_SECRET"); exists {
		cfg.Global.ApiSecret = value
	}
	if value, exists := os.LookupEnv("EXOSCALE_API_CREDENTIALS_FILE"); exists {
		cfg.Global.ApiCredentialsFile = value
	}
	if value, exists := os.LookupEnv("EXOSCALE_API_ENVIRONMENT"); exists {
		cfg.Global.ApiEnvironment = value
	}

	// Defaults
	if cfg.Global.ApiEnvironment == "" {
		cfg.Global.ApiEnvironment = defaultComputeEnvironment
	}

	return cfg, nil
}
