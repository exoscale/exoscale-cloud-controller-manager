package exoscale

import (
	"fmt"
	"os"

	"github.com/exoscale/egoscale"
)

const defaultComputeEndpoint = "https://api.exoscale.com/v1"

func newExoscaleClient() (*egoscale.Client, error) {
	// an attempt to mimic existing behaviours
	envEndpoint := readFromEnv(
		"EXOSCALE_ENDPOINT",
		"EXOSCALE_COMPUTE_ENDPOINT",
		"CLOUDSTACK_ENDPOINT",
	)

	envKey := readFromEnv(
		"EXOSCALE_KEY",
		"EXOSCALE_API_KEY",
		"CLOUDSTACK_KEY",
		"CLOUSTACK_API_KEY",
	)

	envSecret := readFromEnv(
		"EXOSCALE_SECRET",
		"EXOSCALE_API_SECRET",
		"EXOSCALE_SECRET_KEY",
		"CLOUDSTACK_SECRET",
		"CLOUDSTACK_SECRET_KEY",
	)

	if envEndpoint == "" {
		envEndpoint = defaultComputeEndpoint
	}

	if envKey == "" || envSecret == "" {
		return nil, fmt.Errorf("configuration missing for API Key %q or Secret Key %q", envKey, envSecret)
	}

	egoscale.UserAgent = fmt.Sprintf("Exoscale-K8s-Cloud-Controller/%s %s", versionString, egoscale.UserAgent)

	return egoscale.NewClient(envEndpoint, envKey, envSecret), nil
}

// readFromEnv is a os.Getenv on steroids
func readFromEnv(keys ...string) string {
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
	}
	return ""
}
