package container

import (
	"fmt"

	"github.com/densify-dev/container-data-collection/internal/common"
)

const (
	Jvm             = "jvm"
	heap            = "heap"
	non             = "non"
	JvmRuntimeName  = "java"
	jvmRuntimeInfo  = "jvmRuntimeInfo"
	versionLabel    = "process_runtime_version"
	versionLabelJmx = "version"
	descLabel       = "process_runtime_description"
	nameLabel       = "process_runtime_name"
	nameLabelJmx    = "runtime"
	vendorLabel     = "vendor"
	heapInit        = "heapInit"
	heapMax         = "heapMax"
	mib             = "mib"
	buffer          = "buffer"
	post            = "post"
	gc              = "gc"
	thread          = "thread"
	overhead        = "overhead"
	pct             = "pct"
)

type JvmRuntimeDetails struct {
	RuntimeProcessFields
	Description string  `json:"description,omitempty"`
	Name        string  `json:"name,omitempty"`
	Vendor      string  `json:"vendor,omitempty"`
	heapInitMib float64 `json:"-"`
	heapMaxMib  float64 `json:"-"`
}

func (jrtd *JvmRuntimeDetails) runtimeDetails() {}

func jvmRuntimeDetails(r *Runtime) (jrtd *JvmRuntimeDetails, ok bool) {
if r.IsValid() && r.Name == JvmRuntimeName && r.RuntimeDetails != nil {
		jrtd, ok = r.RuntimeDetails.Data.(*JvmRuntimeDetails)
	}
	return
}

var (
	heapUsed      = common.CamelCase(Jvm, heap, common.Used, mib)
	nonHeapUsed   = common.CamelCase(Jvm, non, heap, common.Used, mib)
	buffers       = common.Plural(buffer)
	buffersUsed   = common.CamelCase(Jvm, buffers, common.Used, mib)
	postGcUsed    = common.CamelCase(Jvm, post, gc, common.Used, mib)
	threadCount   = common.CamelCase(Jvm, thread, common.Count)
	gcOverheadPct = common.CamelCase(Jvm, gc, overhead, pct)
)

var jvmQueries = map[string]map[string]string{
	common.OtelJavaAgent: {
		jvmRuntimeInfo: `target_info{telemetry_sdk_language="java"}`,
		heapInit:       `sum by (namespace, pod, container) (max by (namespace, pod, container, jvm_memory_type, jvm_memory_pool_name) (jvm_memory_init_bytes{jvm_memory_type="heap"}))`,
		heapMax:        `sum by (namespace, pod, container) (max by (namespace, pod, container, jvm_memory_type, jvm_memory_pool_name) (jvm_memory_limit_bytes{jvm_memory_type="heap"}))`,
		heapUsed:       `sum by (namespace, pod, container) (max by (namespace, pod, container, jvm_memory_type, jvm_memory_pool_name) (jvm_memory_used_bytes{jvm_memory_type="heap"}))`,
		nonHeapUsed:    `sum by (namespace, pod, container) (max by (namespace, pod, container, jvm_memory_type, jvm_memory_pool_name) (jvm_memory_used_bytes{jvm_memory_type="non_heap"}))`,
		buffersUsed:    `sum by (namespace, pod, container) (max by (namespace, pod, container, jvm_buffer_pool_name) (jvm_buffer_memory_used_bytes{}))`,
		postGcUsed:     `sum by (namespace, pod, container) (max by (namespace, pod, container, jvm_memory_type, jvm_memory_pool_name) (jvm_memory_used_after_last_gc_bytes{}))`,
		threadCount:    `sum by (namespace, pod, container) (max by (namespace, pod, container, jvm_thread_daemon, jvm_thread_state) (jvm_thread_count{}))`,
		gc:             "jvm_gc_duration_seconds_sum",
	},
	common.JmxExporter: {
		jvmRuntimeInfo: common.LabelReplace(common.LabelReplace("jvm_runtime_info{}", nameLabel, nameLabelJmx, common.HasValue), versionLabel, versionLabelJmx, common.HasValue),
		heapInit:       `max by (namespace, pod, container) (jvm_memory_init_bytes{area="heap"})`,
		heapMax:        `max by (namespace, pod, container) (jvm_memory_max_bytes{area="heap"})`,
		heapUsed:       `max by (namespace, pod, container) (jvm_memory_used_bytes{area="heap"})`,
		nonHeapUsed:    `max by (namespace, pod, container) (jvm_memory_used_bytes{area="nonheap"})`,
		buffersUsed:    `sum by (namespace, pod, container) (max by (namespace, pod, container, pool) (jvm_buffer_pool_used_bytes{}))`,
		postGcUsed:     `sum by (namespace, pod, container) (max by (namespace, pod, container, pool) (jvm_memory_pool_collection_used_bytes{}))`,
		threadCount:    `max by (namespace, pod, container) (jvm_threads_current{})`,
		gc:             "jvm_gc_collection_seconds_sum",
	},
}

func getGcOverheadQuery(baseMetric string) string {
	return fmt.Sprintf(`sum by (namespace, pod, container) (rate(%s%s[%dm])) * 100`, baseMetric, common.Braces, common.Params.Collection.SampleRate)
}

func getJvmQuery(otelJavaAgent, jmxExporter bool, what string) (qry string) {
	var qs []string
	var inds []string
	if otelJavaAgent {
		inds = append(inds, common.OtelJavaAgent)
	}
	if jmxExporter {
		inds = append(inds, common.JmxExporter)
	}
	for _, ind := range inds {
		if q := jvmQueries[ind][what]; q != common.Empty {
			qs = append(qs, q)
		}
	}
	switch len(qs) {
	case 1:
		qry = fmt.Sprintf("(%s)", qs[0])
case 2:
		qry = fmt.Sprintf("((%s) or on (namespace, pod, container) (%s))", qs[0], qs[1])
	}
	return
}

func getJvmAttributes(mh *metricHolder) {
	otelJavaAgent, jmxExporter := common.HasOtelJavaAgent(range5Min), common.HasJmxExporter(range5Min)
	if !otelJavaAgent && !jmxExporter {
		return
	}
	// jvmRuntimeInfo needs to be the first!
	var metrics = []string{jvmRuntimeInfo, heapInit, heapMax}
	for _, metric := range metrics {
		mh.metric = metric
		if query := getJvmQuery(otelJavaAgent, jmxExporter, metric); query != common.Empty {
			_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getContainerMetric)
		}
	}
}

func getJvmWorkloads(wq *workloadQuery) {
	otelJavaAgent, jmxExporter := common.HasOtelJavaAgent(range5Min), common.HasJmxExporter(range5Min)
	if !otelJavaAgent && !jmxExporter {
		return
	}
	hadSuffix := wq.hasSuffix
	wq.hasSuffix = false
	suffixes := map[string]string{
		common.Avg: " / (1024 * 1024)",
		common.Max: " / (1024 * 1024)",
	}
	metrics := []string{heapUsed, nonHeapUsed, buffersUsed, postGcUsed}
	getAvgMaxSeparateQueries(wq, jvmQueryMap(otelJavaAgent, jmxExporter, suffixes, metrics))
	suffixes = map[string]string{
		common.Avg: common.Empty,
		common.Max: common.Empty,
	}
	metrics = []string{threadCount}
	getAvgMaxSeparateQueries(wq, jvmQueryMap(otelJavaAgent, jmxExporter, suffixes, metrics))
	updateJvmQueries()
	wq.metricName = gcOverheadPct
	wq.aggregators = suffixes
	wq.baseQuery = getJvmQuery(otelJavaAgent, jmxExporter, gcOverheadPct)
	getWorkload(wq)
	wq.hasSuffix = hadSuffix
}

func jvmQueryMap(otelJavaAgent, jmxExporter bool, suffixes map[string]string, metrics []string) map[string][]*baseWorkloadQuery {
	queryMap := make(map[string][]*baseWorkloadQuery)
	for _, agg := range aggregators {
		for _, mName := range metrics {
			metric := getJvmQuery(otelJavaAgent, jmxExporter, mName)
			addToQueryMap(queryMap, mName, agg, metric, suffixes[agg], 1)
		}
	}
	return queryMap
}

func updateJvmQueries() {
	for _, v := range jvmQueries {
		v[gcOverheadPct] = getGcOverheadQuery(v[gc])
	}
}
