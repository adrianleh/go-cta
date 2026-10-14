package cta

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetPredictionsByVehicleIdsAllowingErrors(t *testing.T) {
	// One bus has predictions, the other is laying over - the API returns both in one response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("vid") != "1,2" {
			t.Errorf("unexpected vid query %q", r.URL.Query().Get("vid"))
		}
		_, _ = w.Write([]byte(`{"bustime-response": {
			"prd": [{"vid": "1", "stpid": "1425", "prdtm": "1768347725", "typ": "A"}],
			"error": [{"vid": "2", "msg": "No arrival times"}]
		}}`))
	}))
	defer server.Close()

	c, err := newClient(server.URL, "key")
	if err != nil {
		t.Fatal(err)
	}
	bc := &BusClient{c: c}

	preds, errs, err := bc.GetPredictionsByVehicleIdsAllowingErrors(t.Context(), []string{"1", "2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*preds) != 1 || (*preds)[0].StopId != "1425" {
		t.Errorf("unexpected predictions %+v", *preds)
	}
	if len(errs) != 1 || errs[0].Vid != "2" {
		t.Errorf("unexpected errors %+v", errs)
	}

	// The strict version still fails on any error
	if _, err := bc.GetPredictionsByVehicleIds(t.Context(), []string{"1", "2"}); err == nil {
		t.Errorf("expected an error from the strict call")
	}
}
