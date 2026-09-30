package exoscale

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	v3 "github.com/exoscale/egoscale/v3"
	"github.com/exoscale/egoscale/v3/credentials"
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

func Test_validateCredentials(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{name: "valid", status: http.StatusOK, body: `{"load-balancers":[]}`},
		{name: "invalid key", status: http.StatusForbidden, body: `{"message":"Invalid key or request signature"}`, wantErr: true},
		{name: "unauthorized", status: http.StatusUnauthorized, body: `{"message":"Unauthorized"}`, wantErr: true},
		// Valid credentials, but the IAM role doesn't allow listing NLBs: don't reject them.
		{name: "operation not allowed", status: http.StatusForbidden, body: `{"message":"Operation list-load-balancers is not allowed"}`},
		{name: "other error", status: http.StatusConflict, body: `{"message":"Conflict"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/load-balancer", r.URL.Path)
				require.NotEmpty(t, r.Header.Get("Authorization"), "request should be signed")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client, err := v3.NewClient(
				credentials.NewStaticCredentials(testAPIKey, testAPISecret),
				v3.ClientOptWithEndpoint(v3.Endpoint(server.URL)),
				v3.ClientOptWithHTTPClient(&http.Client{}),
			)
			require.NoError(t, err)

			err = validateCredentials(context.Background(), client)
			if tt.wantErr {
				require.ErrorContains(t, err, "invalid Exoscale API credentials")
				return
			}
			require.NoError(t, err)

			// switchZoneCallback validates the credentials too.
			_, err = switchZoneCallback(context.Background(), client, "")
			require.NoError(t, err)
		})
	}
}
