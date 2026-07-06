package driver

import "testing"

func assertMetricValue(t *testing.T, metric MetricValue, status, unit string, value float64) {
	t.Helper()
	if metric.Status != status || metric.Unit != unit || metric.Value == nil || *metric.Value != value {
		t.Fatalf("metric = %#v, want status=%s unit=%s value=%v", metric, status, unit, value)
	}
}
