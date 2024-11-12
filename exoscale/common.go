package exoscale

import (
	"context"
	"errors"
	"strings"

	v3 "github.com/exoscale/egoscale/v3"
)

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
