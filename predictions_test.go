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
			"prd": [{"vid": "1", "stpid": "1425", "prdtm": "20260114 19:08:05", "typ": "A"}],
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

func TestPredictionTimesUseBusTrackerFormat(t *testing.T) {
	p := Prediction{Timestamp: "20261014 19:07:12", PredictedTime: "20261014 19:08:00"}
	at, err := p.GetPredictedTime()
	if err != nil {
		t.Fatal(err)
	}
	if got := at.Format("2006-01-02 15:04:05 MST"); got != "2026-10-14 19:08:00 CDT" {
		t.Errorf("got %s", got)
	}
	ts, err := p.GetTimestamp()
	if err != nil || at.Sub(ts).Seconds() != 48 {
		t.Errorf("timestamp %v err %v", ts, err)
	}
}
