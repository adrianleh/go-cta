package cta

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTransportErrorsDontLeakApiKey(t *testing.T) {
	// A server that's gone: the request fails before any response, and Go's error quotes the full URL
	server := httptest.NewServer(nil)
	server.Close()

	c, err := newClient(server.URL, "supersecretkey")
	if err != nil {
		t.Fatal(err)
	}
	bc := &BusClient{c: c}
	_, err = bc.GetRoutes(t.Context())
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "supersecretkey") {
		t.Errorf("API key leaked in error: %v", err)
	}
	if !strings.Contains(err.Error(), "key=[REDACTED]") {
		t.Errorf("expected the redacted URL in the error, got: %v", err)
	}
}
