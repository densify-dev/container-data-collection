package common

import (
	"strings"
	"testing"
)

const exportedReplaceSuffix = `, "namespace", "$1", "exported_namespace", "(.+)"), "pod", "$1", "exported_pod", "(.+)"), "container", "$1", "exported_container", "(.+)")`

func TestExportedLabelsReplace(t *testing.T) {
	got := ExportedLabelsReplace("m{}")
	want := `label_replace(label_replace(label_replace(m{}` + exportedReplaceSuffix
	if got != want {
		t.Fatalf("ExportedLabelsReplace() = %q, want %q", got, want)
	}
}

func TestGpuAggOverTimeQueryExportedLabelReplace(t *testing.T) {
	q := "DCGM_FI_DEV_GPU_UTIL{}"
	plain := &GpuQueryGenerator{}
	if got := plain.GpuAggOverTimeQuery(q, Avg); strings.Contains(got, ExportedNamespace) {
		t.Fatalf("unexpected exported_ label replace without ExportedLabelReplace: %q", got)
	}
	gqg := &GpuQueryGenerator{ExportedLabelReplace: true, DcgmLabelReplace: true}
	got := gqg.GpuAggOverTimeQuery(q, Avg)
	// exported_ replace wraps the over-time query, the node replace wraps both
	inner := ExportedLabelsReplace(plain.GpuAggOverTimeQuery(q, Avg))
	if want := DcgmExporterLabelReplace(inner); got != want {
		t.Fatalf("GpuAggOverTimeQuery() = %q, want %q", got, want)
	}
}

func TestExportedPercentQuerySuffix(t *testing.T) {
	on := []string{Namespace, Pod, Container}
	if got, want := PercentQuerySuffix("m", nil, on...), ` * on (namespace,pod,container) m{} / 100`; got != want {
		t.Fatalf("PercentQuerySuffix() = %q, want %q", got, want)
	}
	got := ExportedPercentQuerySuffix("m", nil, on...)
	want := ` * on (namespace,pod,container) ` + ExportedLabelsReplace("m{}") + ` / 100`
	if got != want {
		t.Fatalf("ExportedPercentQuerySuffix() = %q, want %q", got, want)
	}
}
