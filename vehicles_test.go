package cta

import "testing"

func TestVehicleGetTimestamp(t *testing.T) {
	v := Vehicle{Timestamp: "20260113 17:42:05"}
	ts, err := v.GetTimestamp()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts.Location().String() != "America/Chicago" {
		t.Errorf("expected Chicago time, got %s", ts.Location())
	}
	if ts.Hour() != 17 || ts.Minute() != 42 || ts.Second() != 5 {
		t.Errorf("unexpected time %s", ts)
	}
	unix, err := v.GetUnixTimestamp()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 2026-01-13 17:42:05 CST is 23:42:05 UTC
	if unix != 1768347725 {
		t.Errorf("expected 1768347725, got %d", unix)
	}

	bad := Vehicle{Timestamp: ""}
	if _, err := bad.GetTimestamp(); err == nil {
		t.Errorf("expected error for empty timestamp")
	}
}
