package common

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConditionalQueriesSingleLine(t *testing.T) {
	for unified, queries := range conditionalQueries {
		for i, query := range queries {
			if strings.ContainsAny(query, "\r\n") {
				t.Errorf("unified=%v query=%d contains a newline", unified, i)
			}
			if query != Empty && strings.Count(query, "%s") != 1+i%2 {
				t.Errorf("unified=%v query=%d lost node-group substitution slots: %s", unified, i, query)
			}
		}
	}
}

// These tests evaluate the generated PromQL, rather than only comparing strings.
// Install promtool (from Prometheus) on PATH to run them.
func TestContainerResourceQueriesPromQL(t *testing.T) {
	promtool, err := exec.LookPath("promtool")
	if err != nil {
		t.Skip("promtool is required for PromQL evaluation")
	}
	for _, resource := range []string{Cpu, Memory, NvidiaGpuResource, "ephemeral_storage", "cpu_cores", "memory_bytes"} {
		t.Run(resource, func(t *testing.T) {
			legacy := resource == "cpu_cores" || resource == "memory_bytes"
			unit := 1.0
			if resource == Memory || resource == "memory_bytes" {
				unit = 1024 * 1024
			}
			selector := func(metric string) string {
				if legacy {
					return metric + "_" + resource + "{}"
				}
				return QueryForResource(metric+"{}", resource)
			}
			var series []map[string]any
			add := func(metric, labels string, value float64) {
				series = append(series, map[string]any{
					"series": metric + "{" + labels + "}",
					"values": fmt.Sprintf("%g %g %g", value, value, value),
				})
			}
			identity := func(pod, uid, container string) string {
				return fmt.Sprintf(`namespace="ns",pod="%s",uid="%s",container="%s"`, pod, uid, container)
			}
			phase := func(pod, uid, state string) {
				add("kube_pod_status_phase", fmt.Sprintf(`namespace="ns",pod="%s",uid="%s",phase="%s"`, pod, uid, state), 1)
			}
			request := func(pod, uid, container string, init bool, value float64) {
				prefix := "kube_pod_container_resource_"
				if init {
					prefix = "kube_pod_init_container_resource_"
				}
				for _, kind := range []string{"requests", "limits"} {
					metric := prefix + kind
					labels := identity(pod, uid, container) + `,node="n1"`
					if legacy {
						metric += "_" + resource
					} else {
						labels += fmt.Sprintf(`,resource="%s"`, resource)
					}
					// Duplicate scrape targets must not multiply resource totals.
					add(metric, labels+`,instance="a"`, value*unit)
					add(metric, labels+`,instance="b"`, value*unit)
				}
			}
			phase("active", "new", "Running")
			request("active", "new", "app", false, 2)
			request("active", "new", "waiting", false, 3)
			// No status_running metrics exist; waiting and missing termination
			// metrics must retain requests. An old UID must not exclude the app.
			add("kube_pod_container_status_terminated", identity("active", "old", "app"), 1)
			for _, init := range []bool{false, true} {
				prefix := "kube_pod_container_"
				if init {
					prefix = "kube_pod_init_container_"
				}
				for _, status := range []string{"status_terminated", "status_terminated_reason"} {
					container := prefix + status
					request("active", "new", container, init, 100)
					add(prefix+status, identity("active", "new", container), 1)
					if init {
						add(prefix+"info", identity("active", "new", container)+`,restart_policy="Always"`, 1)
					}
				}
			}
			for _, state := range []string{"Succeeded", "Failed", "Pending"} {
				phase(state, state, state)
				request(state, state, "app", false, 1000)
				request(state, state, "sidecar", true, 1000)
				add("kube_pod_init_container_info", identity(state, state, "sidecar")+`,restart_policy="Always"`, 1)
			}
			phase("active", "old", "Succeeded")
			request("active", "old", "app", false, 1000)
			request("unknown", "unknown", "app", false, 1000)
			request("active", "new", "sidecar", true, 4)
			add("kube_pod_init_container_info", identity("active", "new", "sidecar")+`,restart_policy="Always",instance="a"`, 1)
			add("kube_pod_init_container_info", identity("active", "new", "sidecar")+`,restart_policy="Always",instance="b"`, 1)
			request("active", "new", "ordinary-init", true, 100)
			add("kube_pod_init_container_info", identity("active", "new", "ordinary-init")+`,restart_policy="Never"`, 1)
			request("active", "new", "zero-info", true, 100)
			add("kube_pod_init_container_info", identity("active", "new", "zero-info")+`,restart_policy="Always"`, 0)
			allocatable := selector("kube_node_status_allocatable")
			allocatable = strings.Replace(allocatable, "{", `{node="n1",`, 1)
			series = append(series, map[string]any{"series": allocatable, "values": fmt.Sprintf("%g %g %g", 20*unit, 20*unit, 20*unit)})
			var expressions []map[string]any
			check := func(query, labels string, value float64) {
				if strings.ContainsAny(query, "\r\n") {
					t.Fatalf("query contains a newline: %q", query)
				}
				for _, at := range []string{"0m", "5m", "10m"} {
					expressions = append(expressions, map[string]any{
						"expr": query, "eval_time": at,
						"exp_samples": []map[string]any{{"labels": labels, "value": value}},
					})
				}
			}
			for _, kind := range []string{"requests", "limits"} {
				query := FilterTerminatedContainers(selector("kube_pod_container_resource_"+kind), selector("kube_pod_init_container_resource_"+kind))
				check("sum("+query+") by (node)", `{node="n1"}`, 9*unit)
			}
			if resource != "ephemeral_storage" {
				total := 9.0
				if legacy {
					total = 5 // Legacy shared queries retain their regular-only inputs.
				}
				suffix := Empty
				if unit != 1 {
					suffix = toMiB
				}
				check(fmt.Sprintf(nodeRequestsQuery(resource, suffix), Empty), "{}", total)
				check(fmt.Sprintf(nodeReservationPercentQuery(resource), Empty, Empty), "{}", total*5)
				// Node-group joins must occur after per-container deduplication.
				add("kube_node_labels", `node="n1",label_pool="workers"`, 1)
				join := ` * on (node) group_left(label_pool) kube_node_labels{}) by (label_pool`
				check(fmt.Sprintf(nodeRequestsQuery(resource, suffix), join), `{label_pool="workers"}`, total)
				check(fmt.Sprintf(nodeReservationPercentQuery(resource), join, join), `{label_pool="workers"}`, total*5)
			}
			data, err := yaml.Marshal(map[string]any{"evaluation_interval": "5m", "tests": []map[string]any{{
				"interval": "5m", "input_series": series, "promql_expr_test": expressions,
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
		})
	}
}
