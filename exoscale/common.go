package exoscale

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	v3 "github.com/exoscale/egoscale/v3"
)

const metadataEndpoint = "http://metadata.exoscale.com/1.0/meta-data/"

func (p *cloudProvider) computeInstanceByProviderID(ctx context.Context, providerID string) (*v3.Instance, error) {
	id, err := formatProviderID(providerID)
	if err != nil {
		return nil, err
	}

	return p.client.GetInstance(ctx, id)
}

func formatProviderID(providerID string) (v3.UUID, error) {
	if providerID == "" {
		return "", errors.New("provider ID cannot be empty")
	}

	return v3.UUID(strings.TrimPrefix(providerID, providerPrefix)), nil
}

func queryInstanceMetadata(key string) (string, error) {
	resp, err := http.Get(metadataEndpoint + key)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	value, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(value), nil
}
