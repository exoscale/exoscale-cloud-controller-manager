package exoscale

import (
	"context"

	v3 "github.com/exoscale/egoscale/v3"
)

func (c *refreshableExoscaleClient) ListSKSClusters(
	ctx context.Context,
) (*v3.ListSKSClustersResponse, error) {
	c.RLock()
	defer c.RUnlock()

	return c.exo.ListSKSClusters(
		ctx,
	)
}
