package exoscale

import (
	"net/http"
	"net/http/httptest"
)

const (
	jsonContentType = "application/json"
)

type response struct {
	code        int
	contentType string
	body        string
}

type testServer struct {
	*httptest.Server
	lastResponse int
	responses    []response
}

func newServer(responses ...response) *testServer {
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
		w.Header().Set("Content-Type", jsonContentType)
		w.WriteHeader(500)
		w.Write([]byte("{}")) // nolint: errcheck
		return
	}
	response := ts.responses[i]
	ts.lastResponse++

	w.Header().Set("Content-Type", response.contentType)
	w.WriteHeader(response.code)
	w.Write([]byte(response.body)) // nolint: errcheck
}
