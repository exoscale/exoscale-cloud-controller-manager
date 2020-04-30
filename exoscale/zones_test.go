package exoscale

import (
	"context"

	cloudprovider "k8s.io/cloud-provider"
)

func (s *ConfigTestSuite) TestGetZoneByProviderID() {
	ctx := context.Background()
	p, ts := newFakeInstanceAPI()
	zones := &zones{p: p}
	defer ts.Close()

	zone, err := zones.GetZoneByProviderID(ctx, "exoscale://8a3a817d-3874-477c-adaf-2b2ce9172528")

	s.Require().Nil(err)

	expectedZone := cloudprovider.Zone{Region: "ch-dk-2"}

	s.Require().Equal(expectedZone, zone)
}

func (s *ConfigTestSuite) TestGetZoneByNodeName() {
	ctx := context.Background()
	p, ts := newFakeInstanceAPI()
	zones := &zones{p: p}
	defer ts.Close()

	zone, err := zones.GetZoneByNodeName(ctx, "k8s-master")

	s.Require().Nil(err)

	expectedZone := cloudprovider.Zone{Region: "ch-dk-2"}

	s.Require().Equal(expectedZone, zone)
}
