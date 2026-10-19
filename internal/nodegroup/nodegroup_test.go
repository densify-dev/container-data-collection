package nodegroup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v3"
)

func TestNodeGroupResourceQueriesPromQL(t *testing.T) {
	promtool, err := exec.LookPath("promtool")
	if err != nil {
		t.Skip("promtool is required for PromQL evaluation")
	}
	previous := roleEng
	roleEng = &roleEngine{coreQuery: buildCoreQuery([]string{"control-plane", "master", "infra", "worker"})}
	t.Cleanup(func() { roleEng = previous })
	var series, expressions []map[string]any
	add := func(metric, labels string, value float64) {
		series = append(series, map[string]any{"series": metric + "{" + labels + "}", "values": fmt.Sprint(value)})
	}
	groups := map[string]int{"control-plane": 1, "master": 1, "infra": 1, "worker": 2, "other": 1}
	for group, count := range groups {
		for i := 0; i < count; i++ {
			node := fmt.Sprintf("%s-%d", group, i)
			add("kube_node_role", fmt.Sprintf(`node="%s",role="%s"`, node, group), 1)
			add("kube_node_labels", fmt.Sprintf(`node="%s",label_pool="%s"`, node, group), 1)
			for _, resource := range []string{common.Cpu, common.Memory} {
				unit, capacity, legacy := 1.0, 4.0, "cpu_cores"
				if resource == common.Memory {
					unit, capacity, legacy = 1024*1024, 1000, "memory_bytes"
				}
				add("kube_node_status_capacity", fmt.Sprintf(`node="%s",resource="%s"`, node, resource), capacity*unit)
				add("kube_node_status_allocatable", fmt.Sprintf(`node="%s",resource="%s"`, node, resource), capacity*unit)
				for _, qualifier := range []string{"requests", "limits"} {
					for _, state := range []string{"Running", "Succeeded", "Failed", "Pending", "terminated"} {
						pod := node + "-" + state
						identity := fmt.Sprintf(`namespace="ns",pod="%s",uid="%s"`, pod, pod)
						phase := state
						if state == "terminated" {
							phase = "Running"
						}
						// Add phase and status metrics only once per pod.
						if resource == common.Cpu && qualifier == "requests" {
							add("kube_pod_status_phase", identity+fmt.Sprintf(`,phase="%s"`, phase), 1)
							add("kube_pod_init_container_info", identity+`,container="sidecar",restart_policy="Always"`, 1)
							if state == "terminated" {
								add("kube_pod_container_status_terminated", identity+`,container="app"`, 1)
								add("kube_pod_init_container_status_terminated", identity+`,container="sidecar"`, 1)
							}
						}
						value := 0.25
						if resource == common.Memory {
							value = 100
						}
						for _, instance := range []string{"a", "b"} {
							labels := identity + fmt.Sprintf(`,node="%s",instance="%s"`, node, instance)
							add("kube_pod_container_resource_"+qualifier, labels+fmt.Sprintf(`,container="app",resource="%s"`, resource), value*unit)
							add("kube_pod_container_resource_"+qualifier+"_"+legacy, labels+`,container="app"`, value*unit)
							add("kube_pod_init_container_resource_"+qualifier, labels+fmt.Sprintf(`,container="sidecar",resource="%s"`, resource), value*unit/2)
						}
					}
				}
			}
		}
	}
	for _, feature := range []clusterFeature{&labelFeature{labelNames: []model.LabelName{"label_pool"}}, &roleFeature{}} {
		label := feature.LabelNames()[0]
		check := func(query string, extraLabels string, perNode float64) {
			var samples []map[string]any
			for group, count := range groups {
				samples = append(samples, map[string]any{
					"labels": fmt.Sprintf(`{%s="%s"%s}`, label, group, extraLabels), "value": perNode * float64(count),
				})
			}
			expressions = append(expressions, map[string]any{"expr": query, "eval_time": "0m", "exp_samples": samples})
		}
		configSuffix := QuerySuffixFmt(feature, common.ConfigSt, true)
		metricSuffix := generateQuery(QuerySuffixFmt(feature, common.Metric, false), label, func(s string) string { return s }) + ")"
		for _, qualifier := range qualifiers {
			for _, unified := range []bool{true, false} {
				for resource, core := range resourceCoreQueries[qualifier][unified] {
					value := 250.0
					if resource == common.Memory {
						value = 100
					}
					if unified {
						value *= 1.5
					}
					query := fmt.Sprintf("sum(sum(%s%s) by (node)%s", core, operands[resource], configSuffix)
					check(generateQuery(query, label, func(s string) string { return s }), "", value)
				}
			}
		}
		for _, resource := range []string{common.Cpu, common.Memory} {
			capacity, request, suffix := 4.0, 0.375, ""
			if resource == common.Memory {
				capacity, request, suffix = 1000*1024*1024, 150, "/1024/1024"
			}
			core := resourceCoreQueries[common.Request][true][resource]
			requests := fmt.Sprintf("sum(sum(%s%s) by (node)%s", core, suffix, metricSuffix)
			check(requests, "", request)
			allocatable := fmt.Sprintf(`sum(sum(kube_node_status_allocatable{resource="%s"}) by (node)%s`, resource, metricSuffix)
			rawRequests := fmt.Sprintf("sum(sum(%s) by (node)%s", core, metricSuffix)
			// Reservation is a ratio, so each group's size cancels out.
			query := "100 * " + rawRequests + " / " + allocatable
			var samples []map[string]any
			percent := 9.375
			if resource == common.Memory {
				percent = 15
			}
			for group := range groups {
				samples = append(samples, map[string]any{"labels": fmt.Sprintf(`{%s="%s"}`, label, group), "value": percent})
			}
			expressions = append(expressions, map[string]any{"expr": query, "eval_time": "0m", "exp_samples": samples})
			capacityQuery := fmt.Sprintf(`sum(kube_node_status_capacity{resource="%s"}%s`, resource, QuerySuffixFmt(feature, common.ConfigSt, true, common.Resource))
			check(generateQuery(capacityQuery, label, func(s string) string { return s }, common.Resource), fmt.Sprintf(`,resource="%s"`, resource), capacity)
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
