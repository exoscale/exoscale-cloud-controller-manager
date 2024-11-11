package exoscale

import (
	"bytes"
	"io"
	"net/http"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	cloudprovider "k8s.io/cloud-provider"
)

type exoscaleMetadataMockTransport struct {
	res *http.Response
	err error
}

func (t *exoscaleMetadataMockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.res.Request = req
	return t.res, t.err
}

func (ts *exoscaleCCMTestSuite) TestGetZone() {
	defaultHTTPClient := http.DefaultClient
	defer func() { http.DefaultClient = defaultHTTPClient }()

	http.DefaultClient = &http.Client{Transport: &exoscaleMetadataMockTransport{
		res: &http.Response{
			Status:        "200 ok",
			StatusCode:    200,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Header:        http.Header{},
			Body:          io.NopCloser(bytes.NewBufferString(testZone)),
			ContentLength: int64(len(testZone)),
		},
	}}

	expected := cloudprovider.Zone{Region: testZone}
	actual, err := ts.p.zones.GetZone(ts.p.ctx)
	ts.Require().NoError(err)
	ts.Require().Equal(expected, actual)
}

func (ts *exoscaleCCMTestSuite) TestGetZoneByProviderID() {
	expected := cloudprovider.Zone{Region: testZone}

	actual, err := ts.p.zones.GetZoneByProviderID(ts.p.ctx, "")
	ts.Require().NoError(err)
	ts.Require().Equal(expected, actual)
}

func (ts *exoscaleCCMTestSuite) TestGetZoneByNodeName() {
	ts.p.kclient = fake.NewSimpleClientset(&v1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: testInstanceName,
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: testInstanceID.String(),
			},
		},
	})

	expected := cloudprovider.Zone{Region: testZone}

	actual, err := ts.p.zones.GetZoneByNodeName(ts.p.ctx, types.NodeName(testInstanceName))
	ts.Require().NoError(err)
	ts.Require().Equal(expected, actual)
}

func (ts *exoscaleCCMTestSuite) TestGetZoneByProviderID_overrideExternal() {
	expected := cloudprovider.Zone{Region: testInstanceOverrideExternalRegion}

	actual, err := ts.p.zones.GetZoneByProviderID(ts.p.ctx, testInstanceOverrideRegexpProviderID)
	ts.Require().NoError(err)
	ts.Require().Equal(expected, actual)
}

func (ts *exoscaleCCMTestSuite) TestGetZoneByNodeName_overrideExternal() {
	expected := cloudprovider.Zone{Region: testInstanceOverrideExternalRegion}

	actual, err := ts.p.zones.GetZoneByNodeName(ts.p.ctx, types.NodeName(testInstanceOverrideRegexpNodeName))
	ts.Require().NoError(err)
	ts.Require().Equal(expected, actual)
}
