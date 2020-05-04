package exoscale

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	cloudprovider "k8s.io/cloud-provider"
)

func TestGetZoneByProviderID(t *testing.T) {
	ctx := context.Background()
	p, ts := newMockInstanceAPI()
	zones := &zones{p: p}
	defer ts.Close()

	zone, err := zones.GetZoneByProviderID(ctx, "exoscale://8a3a817d-3874-477c-adaf-2b2ce9172528")

	require.NoError(t, err)
	require.NotNil(t, zone)

	expectedZone := cloudprovider.Zone{Region: "ch-dk-2"}

	require.Equal(t, expectedZone, zone)
}

func TestGetZoneByNodeName(t *testing.T) {
	ctx := context.Background()
	p, ts := newMockInstanceAPI()
	zones := &zones{p: p}
	defer ts.Close()

	zone, err := zones.GetZoneByNodeName(ctx, "k8s-master")

	require.NoError(t, err)
	require.NotNil(t, zone)

	expectedZone := cloudprovider.Zone{Region: "ch-dk-2"}

	require.Equal(t, expectedZone, zone)
}
