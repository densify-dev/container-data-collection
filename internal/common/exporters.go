package common

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	nnet "github.com/densify-dev/net-utils/network"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

const (
	cadvisor         = "cadvisor"
	nodeExporter     = "node-exporter"
	ksm              = "kube-state-metrics"
	ossm             = "openshift-state-metrics"
	Dcgm             = "dcgm-exporter"
	ephemeralStorage = "k8s-ephemeral-storage-metrics"
	KubexGpu         = "kubex-gpu-process-exporter"
	Beyla            = "beyla"
	CustomMetrics    = "custom-metrics" // may be metrics obtained by recording rules, so not a true exporter
	JmxExporter      = "jmx-exporter"
	OtelJavaAgent    = "otel-java-agent"
)

type exporter struct {
	name           string
	metricsPrefix  string
	repMetric      string
	repLabels      []string
	logAllMetrics  bool
	ignoreJobLabel bool
}

type clusterExporter struct {
	exporter
	promJob              string
	ActualScrapeInterval time.Duration
	UpScrapeInterval     time.Duration
}

func (e *exporter) getPrefix() string {
	return e.metricsPrefix + Underscore
}

func getExporterPrefix(metricName string) string {
	return strings.Split(metricName, Underscore)[0]
}

func (e *exporter) logAllClusterMetrics(cluster string, result model.Matrix) {
	s := make([]string, 0, result.Len())
	for _, ss := range result {
		if mn, f := GetLabelValue(ss, metricName); f {
			s = append(s, mn)
		}
	}
	sort.Strings(s)
	var level LogLevel
	if e.logAllMetrics {
		level = Info
	}
	LogCluster(1, level, ClusterFormat+" exporter=%s detected metrics=%v", cluster, true, cluster, e.name, s)
}

func (e *exporter) scrapeIntervalFromRepQuery(cluster string, result model.Matrix) {
	l := len(exporters)
	if len(clusterExporters[cluster]) == 0 {
		clusterExporters[cluster] = make(map[string]*clusterExporter, l)
	}
	if len(clusterExportersByJob[cluster]) == 0 {
		clusterExportersByJob[cluster] = make(map[string][]*clusterExporter, l)
	}
	for _, ss := range result {
		jobName := GetValue(ss, Job)
		ce := &clusterExporter{exporter: *e, promJob: jobName}
		setScrapeInterval(&ce.ActualScrapeInterval, ss, cluster, ce.name, "rep metric")
		clusterExporters[cluster][e.metricsPrefix] = ce
		if jobName != Empty {
			clusterExportersByJob[cluster][jobName] = append(clusterExportersByJob[cluster][jobName], ce)
		}
	}
}

var exporters = makeExporters()

func makeExporters() map[string]*exporter {
	exps := make(map[string]*exporter, 11)
	addExporter(exps, cadvisor, "container_cpu_usage_seconds_total", []string{Container}, false, false)
	addExporter(exps, nodeExporter, "node_cpu_seconds_total", nil, false, false)
	addExporter(exps, ksm, "kube_pod_info", nil, false, false)
	addExporter(exps, ossm, "openshift_clusterresourcequota_usage", nil, false, false)
	addExporter(exps, Dcgm, "DCGM_FI_DEV_GPU_UTIL", nil, true, false)
	addExporter(exps, ephemeralStorage, "ephemeral_storage_node_available", nil, true, false)
	addExporter(exps, KubexGpu, "kubex_gpu_container_requests", nil, true, false)
	addExporter(exps, Beyla, SurveyInfo, nil, true, true)
	addExporter(exps, CustomMetrics, "custom_container_memory_sizing_bytes", []string{Container}, true, true)
	addExporter(exps, JmxExporter, "jvm_gc_collection_seconds_count", []string{Container}, true, false)
	addExporter(exps, OtelJavaAgent, "jvm_gc_duration_seconds_count", []string{Container}, true, false)
	return exps
}

func addExporter(exps map[string]*exporter, name, repMetric string, repLabels []string, logAllMetrics bool, ignoreJobLabel bool) {
	exps[name] = &exporter{name: name, metricsPrefix: getExporterPrefix(repMetric), repMetric: repMetric, repLabels: repLabels, logAllMetrics: logAllMetrics, ignoreJobLabel: ignoreJobLabel}
}

var clusterExporters = make(map[string]map[string]*clusterExporter)
var clusterExportersByJob = make(map[string]map[string][]*clusterExporter)

const (
	nodeExporterPivotQuery = "max(node_cpu_seconds_total{}) by (node, instance)"
	HasNodeLabel           = "node_label_node_name"     // "node" label is present and has the node name
	HasInstanceLabelPodIp  = "instance_label_pod_ip"    // "node" label is absent, "instance" label has a format of IP address:port
	HasInstanceLabelOther  = "instance_label_node_name" // "node" label is absent, "instance" label has a different format and assumed to be node name
)

var (
	once            sync.Once
	beylaPivotQuery = fmt.Sprintf("max(%s%s) by (%s)", SurveyInfo, Braces, SemconvNodeName)
)

func pivotQuery(query string) string {
	return fmt.Sprintf("max(%s) by (%s)", query, Node)
}

func DetermineExporters(range5Min *v1.Range) {
	once.Do(func() {
		_, _ = CollectAndProcessMetric(nodeExporterPivotQuery, range5Min, determineNodeExporter)
		_, _ = CollectAndProcessMetric(pivotQuery(DcgmExporterLabelReplace("DCGM_FI_DEV_GPU_UTIL{}")), range5Min, determineDcgmExporter)
		_, _ = CollectAndProcessMetric(pivotQuery(EphemeralExporterLabelReplace("ephemeral_storage_node_available{}")), range5Min, determineEphemeralStorageExporter)
		_, _ = CollectAndProcessMetric(pivotQuery("kubex_gpu_container_requests{}"), range5Min, determineKubexGpuExporter)
		_, _ = CollectAndProcessMetric(beylaPivotQuery, range5Min, determineBeylaExporter)
		_, _ = CollectAndProcessMetric(pivotQuery("jvm_gc_collection_seconds_count{}"), range5Min, determineJmxExporter)
		_, _ = CollectAndProcessMetric(pivotQuery("jvm_gc_duration_seconds_count{}"), range5Min, determineOtelJavaAgentExporter)
	})
}

var NodeExporterIndicators = make(map[string][]string)

func determineNodeExporter(cluster string, result model.Matrix) {
	if l := result.Len(); l > 0 {
		ss := result[l-1]
		var indicator string
		var f bool
		if _, f = ss.Metric[Node]; f {
			indicator = HasNodeLabel
		} else {
			var instance model.LabelValue
			if instance, f = ss.Metric[Instance]; f {
				if _, _, err := nnet.ParseAddress(string(instance)); err == nil {
					indicator = HasInstanceLabelPodIp
				} else {
					indicator = HasInstanceLabelOther
				}
			}
		}
		if f {
			NodeExporterIndicators[indicator] = append(NodeExporterIndicators[indicator], cluster)
		}
	}
}

var gpuExporters = make(map[string][]string)

var dcgmExporterIndicators = make(map[string]bool)

func determineDcgmExporter(cluster string, result model.Matrix) {
	determineExporter(cluster, result, dcgmExporterIndicators, Dcgm)
}

var kubexGpuExporterIndicators = make(map[string]bool)

func determineKubexGpuExporter(cluster string, result model.Matrix) {
	determineExporter(cluster, result, kubexGpuExporterIndicators, KubexGpu)
}

var ephemeralStorageExporterIndicators = make(map[string]bool)

func determineEphemeralStorageExporter(cluster string, result model.Matrix) {
	determineExporter(cluster, result, ephemeralStorageExporterIndicators, Empty)
}

var beylaExporterIndicators = make(map[string]bool)

func determineBeylaExporter(cluster string, result model.Matrix) {
	determineExporter(cluster, result, beylaExporterIndicators, Empty)
}

var jmxExporterIndicators = make(map[string]bool)

func determineJmxExporter(cluster string, result model.Matrix) {
	determineExporter(cluster, result, jmxExporterIndicators, Empty)
}

var otelJavaAgentIndicators = make(map[string]bool)

func determineOtelJavaAgentExporter(cluster string, result model.Matrix) {
	determineExporter(cluster, result, otelJavaAgentIndicators, Empty)
}

func determineExporter(cluster string, result model.Matrix, exporterIndicators map[string]bool, gpuExporter string) {
	if l := result.Len(); l > 0 {
		exporterIndicators[cluster] = true
		if gpuExporter != Empty {
			gpuExporters[gpuExporter] = append(gpuExporters[gpuExporter], cluster)
		}
	}
}

func HasNodeExporter(range5Min *v1.Range) bool {
	return hasExporter(NodeExporterIndicators, range5Min)
}

func HasDcgmExporter(range5Min *v1.Range) bool {
	return hasExporter(dcgmExporterIndicators, range5Min)
}

func HasKubexGpuExporter(range5Min *v1.Range) bool {
	return hasExporter(kubexGpuExporterIndicators, range5Min)
}

func hasExporter[K comparable, V any](exporterIndicators map[K]V, range5Min *v1.Range) bool {
	DetermineExporters(range5Min)
	return len(exporterIndicators) > 0
}

func GetGpuExporters(range5Min *v1.Range) map[string][]string {
	DetermineExporters(range5Min)
	return gpuExporters
}

var gpuExportersOrdered = []string{KubexGpu, Dcgm}

func DetermineGpuExporter(range5Min *v1.Range) (s string) {
	ges := GetGpuExporters(range5Min)
	for _, ge := range gpuExportersOrdered {
		if len(ges[ge]) > 0 {
			s = ge
			break
		}
	}
	return
}

func GetGpuExporterType(range5Min *v1.Range, cluster string) (s string) {
	DetermineExporters(range5Min)
	if kubexGpuExporterIndicators[cluster] {
		s = KubexGpu
	} else if dcgmExporterIndicators[cluster] {
		s = Dcgm
	}
	return
}

func HasEphemeralStorageExporter(range5Min *v1.Range) bool {
	return hasExporter(ephemeralStorageExporterIndicators, range5Min)
}

func HasBeylaExporter(range5Min *v1.Range) bool {
	return hasExporter(beylaExporterIndicators, range5Min)
}

func HasJmxExporter(range5Min *v1.Range) bool {
	return hasExporter(jmxExporterIndicators, range5Min)
}

func HasOtelJavaAgent(range5Min *v1.Range) bool {
	return hasExporter(otelJavaAgentIndicators, range5Min)
}
