package exoscale

import (
	"context"
	"errors"
	"strings"

	egoscale "github.com/exoscale/egoscale/v2"
)

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
