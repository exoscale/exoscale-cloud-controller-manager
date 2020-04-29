package exoscale

import (
	"errors"
	"fmt"
	"os"

	"github.com/exoscale/egoscale"
)

const defaultComputeEndpoint = "https://api.exoscale.com/v1"

func newExoscaleClient() (*egoscale.Client, error) {
	envEndpoint := os.Getenv("EXOSCALE_API_ENDPOINT")
	envKey := os.Getenv("EXOSCALE_API_KEY")
	envSecret := os.Getenv("EXOSCALE_API_SECRET")

	if envEndpoint == "" {
		envEndpoint = defaultComputeEndpoint
	}

	if envKey == "" || envSecret == "" {
		return nil, errors.New("incomplete or missing for API credentials")
	}

	egoscale.UserAgent = fmt.Sprintf("Exoscale-K8s-Cloud-Controller/%s %s", versionString, egoscale.UserAgent)

	return egoscale.NewClient(envEndpoint, envKey, envSecret), nil
}
