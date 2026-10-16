package cluster

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/densify-dev/container-data-collection/internal/common"
	"gopkg.in/yaml.v3"
)

func TestClusterResourceQueriesPromQL(t *testing.T) {
	promtool, err := exec.LookPath("promtool")
	if err != nil {
		t.Skip("promtool is required for PromQL evaluation")
	}
	var series, expressions []map[string]any
	add := func(metric, labels string, value float64) {
		series = append(series, map[string]any{"series": metric + "{" + labels + "}", "values": fmt.Sprint(value)})
	}
	for _, state := range []string{"Running", "Succeeded", "Failed", "Pending", "terminated"} {
		identity := fmt.Sprintf(`namespace="ns",pod="%s",uid="%s"`, state, state)
		phase := state
		if state == "terminated" {
			phase = "Running"
		}
		add("kube_pod_status_phase", identity+fmt.Sprintf(`,phase="%s"`, phase), 1)
		add("kube_pod_init_container_info", identity+`,container="sidecar",restart_policy="Always"`, 1)
		if state == "terminated" {
			add("kube_pod_container_status_terminated", identity+`,container="app"`, 1)
			add("kube_pod_init_container_status_terminated", identity+`,container="sidecar"`, 1)
		}
		for _, qualifier := range []string{common.Requests, common.Limits} {
			for _, resource := range []string{common.Cpu, common.Memory} {
				value, legacy := 0.25, "cpu_cores"
				if resource == common.Memory {
					value, legacy = 100*1024*1024, "memory_bytes"
				}
				for _, instance := range []string{"a", "b"} {
					labels := identity + fmt.Sprintf(`,node="n1",instance="%s"`, instance)
					add("kube_pod_container_resource_"+qualifier, labels+fmt.Sprintf(`,container="app",resource="%s"`, resource), value)
					add("kube_pod_container_resource_"+qualifier+"_"+legacy, labels+`,container="app"`, value)
					add("kube_pod_init_container_resource_"+qualifier, labels+fmt.Sprintf(`,container="sidecar",resource="%s"`, resource), value/2)
				}
			}
		}
	}
	for _, qualifier := range []string{common.Requests, common.Limits} {
		expressions = append(expressions, map[string]any{
			"expr": resourceQuery(qualifier, common.Empty, true), "eval_time": "0m",
			"exp_samples": []map[string]any{
				{"labels": `{resource="cpu"}`, "value": 0.375},
				{"labels": `{resource="memory"}`, "value": 150 * 1024 * 1024},
			},
		})
		for _, resource := range []string{common.Cpu, common.Memory} {
			value := 250
			if resource == common.Memory {
				value = 100
			}
			expressions = append(expressions, map[string]any{
				"expr": resourceQuery(qualifier, resource, false), "eval_time": "0m",
				"exp_samples": []map[string]any{{"labels": "{}", "value": value}},
			})
		}
	}
	data, err := yaml.Marshal(map[string]any{"evaluation_interval": "1m", "tests": []map[string]any{{
		"interval": "1m", "input_series": series, "promql_expr_test": expressions,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "queries.yml")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(promtool, "test", "rules", path).CombinedOutput(); err != nil {
		t.Fatalf("promtool: %v\n%s", err, output)
	}
}
