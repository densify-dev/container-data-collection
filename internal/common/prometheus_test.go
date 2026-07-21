package common

import (
	"testing"
	"time"
)

func TestAdjustIntervalToScrapeIntervalNestedOverTime(t *testing.T) {
	cluster := "test-cluster"
	clusterExporters[cluster] = map[string]*clusterExporter{
		"container": {ActualScrapeInterval: 30 * time.Second},
	}
	t.Cleanup(func() { delete(clusterExporters, cluster) })

	query := `max_over_time(irate(container_cpu_usage_seconds_total[1m])[5m:*1])`
	got, si := adjustIntervalToScrapeInterval(cluster, query)
	want := `max_over_time(irate(container_cpu_usage_seconds_total[1m])[5m:30s])`

	if got != want {
		t.Fatalf("adjustIntervalToScrapeInterval() query = %q, want %q", got, want)
	}
	if si != 30*time.Second {
		t.Fatalf("adjustIntervalToScrapeInterval() scrape interval = %v, want %v", si, 30*time.Second)
	}
}

func TestAdjustIntervalToScrapeIntervalNestedOverTimeWithIrateMultiplier(t *testing.T) {
	cluster := "test-cluster"
	clusterExporters[cluster] = map[string]*clusterExporter{
		"container": {ActualScrapeInterval: 30 * time.Second},
	}
	t.Cleanup(func() { delete(clusterExporters, cluster) })

	query := `max_over_time(irate(container_cpu_usage_seconds_total{name!~"k8s_POD_.*"}[*3])[5m:*1])`
	got, si := adjustIntervalToScrapeInterval(cluster, query)
	want := `max_over_time(irate(container_cpu_usage_seconds_total{name!~"k8s_POD_.*"}[1m30s])[5m:30s])`

	if got != want {
		t.Fatalf("adjustIntervalToScrapeInterval() query = %q, want %q", got, want)
	}
	if si != 30*time.Second {
		t.Fatalf("adjustIntervalToScrapeInterval() scrape interval = %v, want %v", si, 30*time.Second)
	}
}

func TestAdjustIntervalToScrapeIntervalMultipliesIrateRange(t *testing.T) {
	cluster := "test-cluster"
	clusterExporters[cluster] = map[string]*clusterExporter{
		"container": {ActualScrapeInterval: 30 * time.Second},
	}
	t.Cleanup(func() { delete(clusterExporters, cluster) })

	query := `irate(container_cpu_usage_seconds_total[*3])`
	got, si := adjustIntervalToScrapeInterval(cluster, query)
	want := `irate(container_cpu_usage_seconds_total[1m30s])`

	if got != want {
		t.Fatalf("adjustIntervalToScrapeInterval() query = %q, want %q", got, want)
	}
	if si != 30*time.Second {
		t.Fatalf("adjustIntervalToScrapeInterval() scrape interval = %v, want %v", si, 30*time.Second)
	}
}

func TestAdjustIntervalToScrapeIntervalLeavesSimpleOverTimeRange(t *testing.T) {
	cluster := "test-cluster"
	clusterExporters[cluster] = map[string]*clusterExporter{
		"container": {ActualScrapeInterval: 30 * time.Second},
	}
	t.Cleanup(func() { delete(clusterExporters, cluster) })

	query := `max_over_time(container_cpu_usage_seconds_total[5m])`
	got, si := adjustIntervalToScrapeInterval(cluster, query)

	if got != query {
		t.Fatalf("adjustIntervalToScrapeInterval() query = %q, want %q", got, query)
	}
	if si != 0 {
		t.Fatalf("adjustIntervalToScrapeInterval() scrape interval = %v, want 0", si)
	}
}

func TestAdjustIntervalToScrapeIntervalNoIntervals(t *testing.T) {
	cluster := "test-cluster"
	clusterExporters[cluster] = map[string]*clusterExporter{
		"container": {ActualScrapeInterval: 30 * time.Second},
	}
	t.Cleanup(func() { delete(clusterExporters, cluster) })

	query := `sum(container_cpu_usage_seconds_total)`
	got, si := adjustIntervalToScrapeInterval(cluster, query)

	if got != query {
		t.Fatalf("adjustIntervalToScrapeInterval() query = %q, want %q", got, query)
	}
	if si != 0 {
		t.Fatalf("adjustIntervalToScrapeInterval() scrape interval = %v, want 0", si)
	}
}

func TestAdjustIntervalToScrapeIntervalIgnoresUnknownRangeFunction(t *testing.T) {
	cluster := "test-cluster"
	clusterExporters[cluster] = map[string]*clusterExporter{
		"container": {ActualScrapeInterval: 30 * time.Second},
	}
	t.Cleanup(func() { delete(clusterExporters, cluster) })

	query := `fake(container_cpu_usage_seconds_total[*3])`
	got, si := adjustIntervalToScrapeInterval(cluster, query)

	if got != query {
		t.Fatalf("adjustIntervalToScrapeInterval() query = %q, want %q", got, query)
	}
	if si != 0 {
		t.Fatalf("adjustIntervalToScrapeInterval() scrape interval = %v, want 0", si)
	}
}

func TestAdjustIntervalToScrapeIntervalOverTimeResolution(t *testing.T) {
	cluster := "test-cluster"
	clusterExporters[cluster] = map[string]*clusterExporter{
		"container": {ActualScrapeInterval: 30 * time.Second},
	}
	t.Cleanup(func() { delete(clusterExporters, cluster) })

	query := `max_over_time(container_cpu_usage_seconds_total[5m:*2])`
	got, si := adjustIntervalToScrapeInterval(cluster, query)
	want := `max_over_time(container_cpu_usage_seconds_total[5m:1m0s])`

	if got != want {
		t.Fatalf("adjustIntervalToScrapeInterval() query = %q, want %q", got, want)
	}
	if si != 30*time.Second {
		t.Fatalf("adjustIntervalToScrapeInterval() scrape interval = %v, want %v", si, 30*time.Second)
	}
}
