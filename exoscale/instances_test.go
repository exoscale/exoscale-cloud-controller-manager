package exoscale

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	egoscale "github.com/exoscale/egoscale/v2"
	"github.com/jarcoal/httpmock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	testInstanceCreatedAt              = time.Now().UTC()
	testInstanceDiskSize         int64 = 10
	testInstanceID                     = new(exoscaleCCMTestSuite).randomID()
	testInstanceIPv6Enabled            = false
	testInstanceManagerID              = new(exoscaleCCMTestSuite).randomID()
	testInstanceName                   = new(exoscaleCCMTestSuite).randomString(10)
	testInstancePublicIPAddress        = "159.100.251.253"
	testInstancePublicIPAddressP       = net.ParseIP(testInstancePublicIPAddress)
	testInstanceState                  = "running"
	testInstanceTemplateID             = new(exoscaleCCMTestSuite).randomID()
	testInstanceTypeAuthorized         = true
	testInstanceTypeCPUs         int64 = 2
	testInstanceTypeFamily             = "standard"
	testInstanceTypeID                 = new(exoscaleCCMTestSuite).randomID()
	testInstanceTypeMemory       int64 = 4294967296
	testInstanceTypeSize               = "medium"
)

func (ts *exoscaleCCMTestSuite) TestNodeAddresses() {
	ts.mockAPIRequest("GET", "/instance/"+testInstanceID, egoscale.Instance{
		CreatedAt:      &testInstanceCreatedAt,
		DiskSize:       &testInstanceDiskSize,
		ID:             &testInstanceID,
		IPv6Enabled:    &testInstanceIPv6Enabled,
		InstanceTypeID: &testInstanceTypeID,
		Manager: &egoscale.InstanceManager{
			ID:   testInstanceManagerID,
			Type: "instance-pool",
		},
		Name:            &testInstanceName,
		PublicIPAddress: &testInstancePublicIPAddressP,
		State:           &testInstanceState,
		TemplateID:      &testInstanceTemplateID,
	}.ToAPIMock())

	ts.provider.kclient = fake.NewSimpleClientset(&v1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: testInstanceName,
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: testInstanceID,
			},
		},
	})

	expected := []v1.NodeAddress{{
		Type:    v1.NodeExternalIP,
		Address: testInstancePublicIPAddress,
	}}

	actual, err := ts.provider.instances.NodeAddresses(context.Background(), types.NodeName(testInstanceName))
	ts.Require().NoError(err)
	ts.Require().Equal(expected, actual)
}

func (ts *exoscaleCCMTestSuite) TestNodeAddressesByProviderID() {
	ts.mockAPIRequest("GET", "/instance/"+testInstanceID, egoscale.Instance{
		CreatedAt:      &testInstanceCreatedAt,
		DiskSize:       &testInstanceDiskSize,
		ID:             &testInstanceID,
		IPv6Enabled:    &testInstanceIPv6Enabled,
		InstanceTypeID: &testInstanceTypeID,
		Manager: &egoscale.InstanceManager{
			ID:   testInstanceManagerID,
			Type: "instance-pool",
		},
		Name:            &testInstanceName,
		PublicIPAddress: &testInstancePublicIPAddressP,
		State:           &testInstanceState,
		TemplateID:      &testInstanceTemplateID,
	}.ToAPIMock())

	ts.provider.kclient = fake.NewSimpleClientset(&v1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: testInstanceName,
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: testInstanceID,
			},
		},
	})

	expected := []v1.NodeAddress{
		{
			Type:    v1.NodeExternalIP,
			Address: testInstancePublicIPAddress,
		},
	}

	actual, err := ts.provider.instances.NodeAddressesByProviderID(context.Background(), providerPrefix+testInstanceID)
	ts.Require().NoError(err)
	ts.Require().Equal(expected, actual)
}

func (ts *exoscaleCCMTestSuite) TestInstanceID() {
	ts.provider.kclient = fake.NewSimpleClientset(&v1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: testInstanceName,
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: testInstanceID,
			},
		},
	})

	actual, err := ts.provider.instances.InstanceID(context.Background(), types.NodeName(testInstanceName))
	ts.Require().NoError(err)
	ts.Require().Equal(actual, testInstanceID)
}

func (ts *exoscaleCCMTestSuite) TestInstanceType() {
	ts.mockAPIRequest("GET", "/instance/"+testInstanceID, egoscale.Instance{
		CreatedAt:      &testInstanceCreatedAt,
		DiskSize:       &testInstanceDiskSize,
		ID:             &testInstanceID,
		IPv6Enabled:    &testInstanceIPv6Enabled,
		InstanceTypeID: &testInstanceTypeID,
		Manager: &egoscale.InstanceManager{
			ID:   testInstanceManagerID,
			Type: "instance-pool",
		},
		Name:            &testInstanceName,
		PublicIPAddress: &testInstancePublicIPAddressP,
		State:           &testInstanceState,
		TemplateID:      &testInstanceTemplateID,
	}.ToAPIMock())
	ts.mockAPIRequest("GET", "/instance-type/"+testInstanceTypeID, egoscale.InstanceType{
		Authorized: &testInstanceTypeAuthorized,
		CPUs:       &testInstanceTypeCPUs,
		Family:     &testInstanceTypeFamily,
		ID:         &testInstanceTypeID,
		Memory:     &testInstanceTypeMemory,
		Size:       &testInstanceTypeSize,
	}.ToAPIMock())

	ts.provider.kclient = fake.NewSimpleClientset(&v1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: testInstanceName,
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: testInstanceID,
			},
		},
	})

	actual, err := ts.provider.instances.InstanceType(context.Background(), types.NodeName(testInstanceName))
	ts.Require().NoError(err)
	ts.Require().Equal(actual, testInstanceTypeSize)
}

func (ts *exoscaleCCMTestSuite) TestInstanceTypeByProviderID() {
	ts.mockAPIRequest("GET", "/instance/"+testInstanceID, egoscale.Instance{
		CreatedAt:      &testInstanceCreatedAt,
		DiskSize:       &testInstanceDiskSize,
		ID:             &testInstanceID,
		IPv6Enabled:    &testInstanceIPv6Enabled,
		InstanceTypeID: &testInstanceTypeID,
		Manager: &egoscale.InstanceManager{
			ID:   testInstanceManagerID,
			Type: "instance-pool",
		},
		Name:            &testInstanceName,
		PublicIPAddress: &testInstancePublicIPAddressP,
		State:           &testInstanceState,
		TemplateID:      &testInstanceTemplateID,
	}.ToAPIMock())

	ts.mockAPIRequest("GET", "/instance-type/"+testInstanceTypeID, egoscale.InstanceType{
		Authorized: &testInstanceTypeAuthorized,
		CPUs:       &testInstanceTypeCPUs,
		Family:     &testInstanceTypeFamily,
		ID:         &testInstanceTypeID,
		Memory:     &testInstanceTypeMemory,
		Size:       &testInstanceTypeSize,
	}.ToAPIMock())

	ts.provider.kclient = fake.NewSimpleClientset(&v1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: testInstanceName,
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: testInstanceID,
			},
		},
	})

	actual, err := ts.provider.instances.InstanceTypeByProviderID(context.Background(), providerPrefix+testInstanceID)
	ts.Require().NoError(err)
	ts.Require().Equal(actual, testInstanceTypeSize)
}

func (ts *exoscaleCCMTestSuite) TestCurrentNodeName() {
	ts.provider.kclient = fake.NewSimpleClientset(&v1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: testInstanceName,
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: testInstanceID,
			},
		},
	})

	actual, err := ts.provider.instances.CurrentNodeName(context.Background(), testInstanceName)
	ts.Require().NoError(err)
	ts.Require().Equal(actual, types.NodeName(testInstanceName))
}

func (ts *exoscaleCCMTestSuite) TestInstanceExistsByProviderID() {
	ts.mockAPIRequest("GET", "/instance/"+testInstanceID, egoscale.Instance{
		CreatedAt:      &testInstanceCreatedAt,
		DiskSize:       &testInstanceDiskSize,
		ID:             &testInstanceID,
		IPv6Enabled:    &testInstanceIPv6Enabled,
		InstanceTypeID: &testInstanceTypeID,
		Manager: &egoscale.InstanceManager{
			ID:   testInstanceManagerID,
			Type: "instance-pool",
		},
		Name:            &testInstanceName,
		PublicIPAddress: &testInstancePublicIPAddressP,
		State:           &testInstanceState,
		TemplateID:      &testInstanceTemplateID,
	}.ToAPIMock())

	ts.provider.kclient = fake.NewSimpleClientset(&v1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: testInstanceName,
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: testInstanceID,
			},
		},
	})

	exists, err := ts.provider.instances.InstanceExistsByProviderID(context.Background(), providerPrefix+testInstanceID)
	ts.Require().NoError(err)
	ts.Require().True(exists)

	// Test with non-existent instance:

	nonExistentID := ts.randomID()

	httpmock.RegisterResponder(
		"GET",
		fmt.Sprintf("%s/instance/%s", testAPIEndpoint, nonExistentID),
		httpmock.NewBytesResponder(http.StatusNotFound, nil),
	)

	exists, err = ts.provider.instances.InstanceExistsByProviderID(context.Background(), providerPrefix+nonExistentID)
	ts.Require().NoError(err)
	ts.Require().False(exists)
}

func (ts *exoscaleCCMTestSuite) TestInstanceShutdownByProviderID() {
	ts.mockAPIRequest("GET", "/instance/"+testInstanceID, egoscale.Instance{
		CreatedAt:      &testInstanceCreatedAt,
		DiskSize:       &testInstanceDiskSize,
		ID:             &testInstanceID,
		IPv6Enabled:    &testInstanceIPv6Enabled,
		InstanceTypeID: &testInstanceTypeID,
		Manager: &egoscale.InstanceManager{
			ID:   testInstanceManagerID,
			Type: "instance-pool",
		},
		Name:            &testInstanceName,
		PublicIPAddress: &testInstancePublicIPAddressP,
		State:           &testInstanceState,
		TemplateID:      &testInstanceTemplateID,
	}.ToAPIMock())

	ts.provider.kclient = fake.NewSimpleClientset(&v1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: testInstanceName,
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: testInstanceID,
			},
		},
	})

	shutdown, err := ts.provider.instances.InstanceShutdownByProviderID(
		context.Background(),
		providerPrefix+testInstanceID,
	)
	ts.Require().NoError(err)
	ts.Require().False(shutdown)
}
