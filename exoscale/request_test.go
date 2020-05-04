package exoscale

import (
	"net/http"
	"net/http/httptest"
)

type testHTTPResponse struct {
	code int
	body string
}

type testServer struct {
	*httptest.Server
	lastResponse int
	responses    []testHTTPResponse
}

func newTestServer(responses ...testHTTPResponse) *testServer {
	mux := http.NewServeMux()

	ts := &testServer{
		httptest.NewServer(mux),
		0,
		responses,
	}

	mux.Handle("/", ts)

	return ts
}

func (ts *testServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	i := ts.lastResponse
	if i >= len(ts.responses) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write([]byte("{}")) // nolint: errcheck
		return
	}
	response := ts.responses[i]
	ts.lastResponse++

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.code)
	w.Write([]byte(response.body)) // nolint: errcheck
}
