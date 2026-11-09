package common

import (
	"strings"
	"testing"
	"time"
)

func TestAdjustIntervalToScrapeIntervalWrappedMetric(t *testing.T) {
	const cluster = "wrapped-metric-test"
	clusterExporters[cluster] = map[string]*clusterExporter{
		"kubex": {ActualScrapeInterval: 30 * time.Second},
	}
	t.Cleanup(func() { delete(clusterExporters, cluster) })
	for _, expr := range []string{
		`label_replace(kubex_gpu_container_memory_bytes{}, "namespace", "$1", "exported_namespace", "(.+)")`,
		`max by (namespace,pod,container) (avg by (namespace,pod,container,container_id) (` +
			ExportedLabelsReplace("kubex_gpu_container_memory_footprint_percent{}") + `))`,
		`(` + ExportedLabelsReplace("kubex_gpu_container_memory_bytes{}") + ` / 1048576)`,
	} {
		query := "max_over_time(" + expr + "[5m:*1])"
		got, si := adjustIntervalToScrapeInterval(cluster, query)
		want := strings.Replace(query, "[5m:*1]", "[5m:30s]", 1)
		if got != want || si != 30*time.Second {
			t.Errorf("adjustIntervalToScrapeInterval(%q) = (%q, %v), want (%q, 30s)", query, got, si, want)
		}
	}
}

func TestMetricNameFromExpression(t *testing.T) {
	for _, test := range []struct{ expression, want string }{
		{`container_cpu_usage_seconds_total{pod="a[5m])"}`, "container_cpu_usage_seconds_total"},
		{`1000 * rate(container_cpu_usage_seconds_total[5m])`, "container_cpu_usage_seconds_total"},
		{`sum without (namespace, pod) (kubex_gpu_container_memory_bytes{})`, "kubex_gpu_container_memory_bytes"},
		{`max by(namespace) (kubex_gpu_container_memory_bytes)`, "kubex_gpu_container_memory_bytes"},
		{`sum(kubex_gpu_container_memory_bytes) by (namespace)`, "kubex_gpu_container_memory_bytes"},
		{`namespace:container_memory_bytes:max{namespace="app"}`, "namespace:container_memory_bytes:max"},
		{`label_replace(vector(1), "dst", "value", "src", "(.+)")`, ""},
		{`{__name__=~"jvm_.*"}`, ""},
	} {
		if got := metricNameFromExpression(test.expression); got != test.want {
			t.Errorf("metricNameFromExpression(%q) = %q, want %q", test.expression, got, test.want)
		}
	}
}

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
