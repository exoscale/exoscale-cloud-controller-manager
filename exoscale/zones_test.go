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

	zone, err := zones.GetZoneByProviderID(ctx, testInstanceProviderID)

	require.NoError(t, err)
	require.NotNil(t, zone)

	expectedZone := cloudprovider.Zone{Region: testInstanceZoneName}

	require.Equal(t, expectedZone, zone)
}

func TestGetZoneByNodeName(t *testing.T) {
	ctx := context.Background()
	p, ts := newMockInstanceAPI()
	zones := &zones{p: p}
	defer ts.Close()

	zone, err := zones.GetZoneByNodeName(ctx, testInstanceName)

	require.NoError(t, err)
	require.NotNil(t, zone)

	expectedZone := cloudprovider.Zone{Region: testInstanceZoneName}

	require.Equal(t, expectedZone, zone)
}
