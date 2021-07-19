package exoscale

import (
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"

	egoscale "github.com/exoscale/egoscale/v2"
)

const metadataEndpoint = "http://metadata.exoscale.com/1.0/meta-data/"

func (p *cloudProvider) computeInstanceByProviderID(ctx context.Context, providerID string) (*egoscale.Instance, error) {
	id, err := formatProviderID(providerID)
	if err != nil {
		return nil, err
	}

	return p.client.GetInstance(ctx, p.zone, id)
}

func formatProviderID(providerID string) (string, error) {
	if providerID == "" {
		return "", errors.New("provider ID cannot be empty")
	}

	return strings.TrimPrefix(providerID, providerPrefix), nil
}

func queryInstanceMetadata(key string) (string, error) {
	resp, err := http.Get(metadataEndpoint + key)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	value, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(value), nil
}
