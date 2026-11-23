package exoscale

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	v3 "github.com/exoscale/egoscale/v3"
	"github.com/exoscale/egoscale/v3/credentials"
)

var testZoneCallback switchZone = func(ctx context.Context, client *v3.Client, zone v3.ZoneName) (*v3.Client, error) {
	return client, nil
}

func (ts *exoscaleCCMTestSuite) Test_newAPIClient_no_config() {
	_, err := newAPIClient(context.Background(), &testConfig_empty.Global, v3.ZoneNameCHGva2, testZoneCallback)
	ts.Require().Error(err)
}

func (ts *exoscaleCCMTestSuite) Test_newAPIClient_credentials() {
	actual, err := newAPIClient(context.Background(), &testConfig_typical.Global, v3.ZoneNameCHGva2, testZoneCallback)
	ts.Require().NoError(err)
	ts.Require().NotNil(actual.exo)
}

func (ts *exoscaleCCMTestSuite) Test_newAPIClient_credentials_file_missing() {
	config := globalConfig{APICredentialsFile: path.Join(ts.T().TempDir(), "credentials.json")}

	_, err := newAPIClient(context.Background(), &config, v3.ZoneNameCHGva2, testZoneCallback)
	ts.Require().ErrorContains(err, "unable to read Exoscale API credentials file")
}

// A credentials file mounted from a Kubernetes Secret is replaced by swapping the ..data symlink:
// the client must sign its next requests with the new key, without being recreated.
func Test_newAPIClient_credentials_file_reload(t *testing.T) {
	const (
		oldKey = "EXO000000000000000000000001"
		newKey = "EXO000000000000000000000002"
		secret = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFG"
	)

	secretDir := t.TempDir()
	writeSecret := func(version, apiKey string) {
		dir := path.Join(secretDir, version)
		require.NoError(t, os.Mkdir(dir, 0o700))
		require.NoError(t, os.WriteFile(
			path.Join(dir, "credentials.json"),
			[]byte(`{"api_key":"`+apiKey+`","api_secret":"`+secret+`","name":"ccm"}`),
			0o600,
		))
		require.NoError(t, os.Symlink(version, path.Join(secretDir, "..data_tmp")))
		require.NoError(t, os.Rename(path.Join(secretDir, "..data_tmp"), path.Join(secretDir, "..data")))
	}
	writeSecret("..v1", oldKey)
	require.NoError(t, os.Symlink("..data/credentials.json", path.Join(secretDir, "credentials.json")))

	var signedWith []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(auth, "credential="+newKey+","):
			signedWith = append(signedWith, newKey)
			_, _ = w.Write([]byte(`{"load-balancers":[]}`))
		case strings.Contains(auth, "credential="+oldKey+","):
			signedWith = append(signedWith, oldKey)
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"Invalid key or request signature"}`))
		default:
			t.Errorf("unexpected Authorization header %q", auth)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := globalConfig{
		APICredentialsFile: path.Join(secretDir, "credentials.json"),
		APIEndpoint:        server.URL,
	}
	client, err := newAPIClient(ctx, &config, v3.ZoneNameCHGva2, testZoneCallback)
	require.NoError(t, err)

	writeSecret("..v2", newKey)

	_, err = client.ListLoadBalancers(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{oldKey, newKey}, signedWith)
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
