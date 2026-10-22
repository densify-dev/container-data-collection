package container

import (
	"testing"

	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/prometheus/common/model"
)

func TestJvmLabelsDoNotAffectContainerLookup(t *testing.T) {
	const cluster = "standard-label-test"
	standard := &model.SampleStream{Metric: model.Metric{
		"namespace": "app", "pod": "pod-1", "container": "main",
	}}
	ownerships[cluster] = make(clusterOwnerships)
	t.Cleanup(func() {
		delete(namespaces, cluster)
		delete(ownerships, cluster)
	})
	addContainerAndOwners(cluster, model.Matrix{standard})

	mh := &metricHolder{metric: jvmRuntimeInfo}
	mh.getContainerMetric(cluster, model.Matrix{&model.SampleStream{Metric: model.Metric{
		"__name__": "target_info", "telemetry_sdk_language": "java",
		"namespace": "app", "pod_name": "pod-1", "container_name": "main",
	}}})

	c, _, ok := getContainer(cluster, standard)
	if !ok {
		t.Fatal("nonstandard JVM labels prevented standard container lookup")
	}
	if len(c.runtimes.runtimes) != 0 {
		t.Fatal("JVM sample with only legacy identity labels matched a container")
	}

	mh.metric = common.Memory
	mh.getContainerMetric(cluster, model.Matrix{&model.SampleStream{
		Metric: standard.Metric,
		Values: []model.SamplePair{{Value: 64 * 1024 * 1024}},
	}})
	if c.memory != 64 {
		t.Fatalf("standard memory metric = %d MiB, want 64", c.memory)
	}

	mh.metric = jvmRuntimeInfo
	mh.getContainerMetric(cluster, model.Matrix{standard})
	if _, ok := c.runtimes.getRuntime(JvmRuntimeName); !ok {
		t.Fatal("JVM sample with standard identity labels did not match its container")
	}
}
