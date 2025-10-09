package exoscale

import (
	"fmt"
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
	if value, exists := os.LookupEnv("EXOSCALE_API_KEY"); exists {
		cfg.Global.APIKey = value
	}
	if value, exists := os.LookupEnv("EXOSCALE_API_SECRET"); exists {
		cfg.Global.APISecret = value
	}
	if value, exists := os.LookupEnv("EXOSCALE_API_CREDENTIALS_FILE"); exists {
		cfg.Global.APICredentialsFile = value
	}
	if value, exists := os.LookupEnv("EXOSCALE_API_ENDPOINT"); exists {
		cfg.Global.APIEndpoint = value
	} else if value, exists := os.LookupEnv("EXOSCALE_API_ENVIRONMENT"); exists {
		if value == "ppapi" {
			cfg.Global.APIEndpoint = fmt.Sprintf("https://%s-ch-gva-2.exoscale.com/compute", value)
		}
	}

	return cfg, nil
}
