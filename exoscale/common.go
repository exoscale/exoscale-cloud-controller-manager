package exoscale

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/exoscale/egoscale"
	v1 "k8s.io/api/core/v1"
)

const metadataEndpoint = "http://metadata.exoscale.com/1.0/meta-data/"

func (c *cloudProvider) computeInstanceByProviderID(ctx context.Context, providerID string) (*egoscale.VirtualMachine, error) {
	id, err := formatProviderID(providerID)
	if err != nil {
		return nil, err
	}

	uuid, err := egoscale.ParseUUID(id)
	if err != nil {
		return nil, err
	}

	r, err := c.client.GetInstance(ctx, uuid)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func nodeAddresses(instance *egoscale.VirtualMachine) ([]v1.NodeAddress, error) {
	nic := instance.DefaultNic()
	if nic == nil {
		return nil, fmt.Errorf("default NIC not found for instance %q", instance.ID.String())
	}

	return []v1.NodeAddress{
		{Type: v1.NodeExternalIP, Address: nic.IPAddress.String()},
	}, nil
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
