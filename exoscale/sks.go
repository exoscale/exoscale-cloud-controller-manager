package exoscale

import (
	"context"

	v3 "github.com/exoscale/egoscale/v3"
)

func (c *apiClient) ListSKSClusters(
	ctx context.Context,
) (*v3.ListSKSClustersResponse, error) {
	return c.exo.ListSKSClusters(
		ctx,
	)
}
