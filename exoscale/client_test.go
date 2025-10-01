package exoscale

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"sync"
	"time"

	v3 "github.com/exoscale/egoscale/v3"
)

var testZoneCallback switchZone = func(ctx context.Context, client *v3.Client, zone v3.ZoneName) (*v3.Client, error) {
	return client, nil
}

func (ts *exoscaleCCMTestSuite) Test_newRefreshableExoscaleClient_no_config() {
	_, err := newRefreshableExoscaleClient(context.Background(), &testConfig_empty.Global, v3.ZoneNameCHGva2, testZoneCallback)
	ts.Require().Error(err)
}

func (ts *exoscaleCCMTestSuite) Test_newRefreshableExoscaleClient_credentials() {
	expected := &refreshableExoscaleClient{
		RWMutex: &sync.RWMutex{}, //nolint:staticcheck
		apiCredentials: exoscaleAPICredentials{
			APIKey:    testAPIKey,
			APISecret: testAPISecret,
		},
	}

	actual, err := newRefreshableExoscaleClient(context.Background(), &testConfig_typical.Global, v3.ZoneNameCHGva2, testZoneCallback)
	ts.Require().NoError(err)
	ts.Require().Equal(expected.apiCredentials, actual.apiCredentials)
	ts.Require().NotNil(actual.exo)
}

func (ts *exoscaleCCMTestSuite) Test_refreshableExoscaleClient_refreshCredentials() {
	testAPICredentials := exoscaleAPICredentials{
		APIKey:    testAPISecret,
		APISecret: testAPISecret,
		Name:      ts.randomString(10),
	}

	jsonAPICredentials, err := json.Marshal(testAPICredentials)
	ts.Require().NoError(err)

	tmpdir, err := os.MkdirTemp(os.TempDir(), "exoscale-ccm")
	ts.Require().NoError(err)
	defer os.RemoveAll(tmpdir)

	testAPICredentialsFile := path.Join(tmpdir, "credentials.json")

	ts.Require().NoError(os.WriteFile(testAPICredentialsFile, jsonAPICredentials, 0o600))

	client := &refreshableExoscaleClient{RWMutex: &sync.RWMutex{}}
	client.refreshCredentialsFromFile(context.Background(), testAPICredentialsFile, v3.ZoneNameCHGva2, testZoneCallback)

	client.RLock()
	defer client.RUnlock()
	ts.Require().Equal(testAPICredentials, client.apiCredentials)
	ts.Require().NotNil(client.exo)
}

func (ts *exoscaleCCMTestSuite) Test_refreshableExoscaleClient_watchCredentialsFile() {
	testAPICredentials := exoscaleAPICredentials{
		APIKey:    testAPISecret,
		APISecret: testAPISecret,
		Name:      ts.randomString(10),
	}

	jsonAPICredentials, err := json.Marshal(testAPICredentials)
	ts.Require().NoError(err)

	tmpdir, err := os.MkdirTemp(os.TempDir(), "exoscale-ccm")
	ts.Require().NoError(err)
	defer os.RemoveAll(tmpdir)

	testAPICredentialsFile := path.Join(tmpdir, "credentials.json")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := &refreshableExoscaleClient{RWMutex: &sync.RWMutex{}}
	go client.watchCredentialsFile(ctx, testAPICredentialsFile, v3.ZoneNameCHGva2, testZoneCallback)

	time.Sleep(1 * time.Second)
	ts.Require().NoError(os.WriteFile(testAPICredentialsFile, jsonAPICredentials, 0o600))
	time.Sleep(1 * time.Second)

	client.RLock()
	defer client.RUnlock()
	ts.Require().Equal(testAPICredentials, client.apiCredentials)
	ts.Require().NotNil(client.exo)
}
