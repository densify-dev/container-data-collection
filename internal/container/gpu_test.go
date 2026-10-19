package container

import (
	"strings"
	"testing"

	"github.com/densify-dev/container-data-collection/internal/common"
)

// exported_ labels are dropped by aggregation, so the replace must be applied to the raw metric inside it
func TestMakeKubexGpuQueryReplacesExportedLabelsBeforeAggregation(t *testing.T) {
	got := makeKubexGpuQuery("kubex_gpu_container_sm_utilization_percent_seconds_total", common.Avg, 5)
	want := `max by (namespace,pod,container) (avg by (namespace,pod,container,container_id) (` +
		common.ExportedLabelsReplace("rate(kubex_gpu_container_sm_utilization_percent_seconds_total{}[5m])") + `))`
	if got != want {
		t.Fatalf("makeKubexGpuQuery() = %q, want %q", got, want)
	}
}

func TestGpuWorkloadQueriesUseExportedLabels(t *testing.T) {
	for _, gwq := range makeGpuWorkloadQueries(5) {
		for ge, q := range gwq.baseQuery {
			if ge != common.KubexGpu || q == common.Empty {
				continue
			}
			if !strings.Contains(q, common.ExportedNamespace) {
				t.Errorf("%s (%s): query %q does not map exported_ labels", gwq.metricName, ge, q)
			}
		}
	}
}

func TestDcgmWorkloadQueriesUseExportedLabels(t *testing.T) {
	if !dqg.ExportedLabelReplace {
		t.Fatal("the DCGM query generator must map exported_ labels")
	}
	q := dqg.GpuAggOverTimeQuery("DCGM_FI_DEV_FB_USED{}", common.Max)
	for _, l := range []string{common.ExportedNamespace, common.ExportedPod, common.ExportedContainer} {
		if !strings.Contains(q, l) {
			t.Errorf("query %q does not map %s", q, l)
		}
	}
}
