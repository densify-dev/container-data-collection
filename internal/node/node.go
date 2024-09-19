package node

import (
	"fmt"
	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/densify-dev/container-data-collection/internal/kubernetes"
	nnet "github.com/densify-dev/net-utils/network"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"sync"
)

// A node structure. Used for storing attributes and config details.
type node struct {
	labelMap                                                                                              map[string]string
	name                                                                                                  string
	providerId                                                                                            string
	k8sVersion                                                                                            string
	netSpeedBytes, cpuCapacity, memCapacity, ephemeralStorageCapacity, podsCapacity, hugepages2MiCapacity int
	cpuAllocatable, memAllocatable, ephemeralStorageAllocatable, podsAllocatable, hugepages2MiAllocatable int
	cpuLimit, cpuRequest, memLimit, memRequest                                                            int
}

// Map that labels and values will be stored in
var nodes = make(map[string]map[string]*node)

// Metrics a global func for collecting node level metrics in prometheus
func Metrics() {
	var query string
	var err error
	range5Min := common.TimeRange()

	// node information/labels
	query = "kube_node_info{}"
	if _, err := common.CollectAndProcessMetric(query, range5Min, createNode); err != nil {
		// error already handled
		return
	}

	// additional config/attribute queries
	query = `kube_node_labels{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, getNodeMetricString)

	query = `kube_node_role{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, getNodeMetricString)

	DetermineNodeExporter(range5Min)

	mh := &metricHolder{name: common.NetSpeedBytes}
	for _, qw := range GetQueryWrappers(&queryWrappers, queryWrappersMap) {
		mh.labelName = qw.MetricField[0]
		query = qw.Query.Wrap(`max(node_network_speed_bytes{device!~"veth.*|docker.*|cilium.*|lxc.*"}) by (node, instance)`)
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getNodeMetric)
	}

	mh.labelName = common.Node
	// Queries the capacity fields of all nodes
	mh.name = common.Capacity
	query = `kube_node_status_capacity{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getNodeMetric)
	if common.Found(indicators, mh.name, false) {
		mh.name = common.CpuCapacity
		query = `kube_node_status_capacity_cpu_cores{}`
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getNodeMetric)

		mh.name = common.MemCapacity
		query = `kube_node_status_capacity_memory_bytes{}`
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getNodeMetric)

		mh.name = common.PodsCapacity
		query = `kube_node_status_capacity_pods{}`
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getNodeMetric)
	}

	mh.name = common.Allocatable
	query = `kube_node_status_allocatable{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getNodeMetric)
	if common.Found(indicators, mh.name, false) {
		mh.name = common.CpuAllocatable
		query = `kube_node_status_allocatable_cpu_cores{}`
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getNodeMetric)

		mh.name = common.MemAllocatable
		query = `kube_node_status_allocatable_memory_bytes{}`
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getNodeMetric)

		mh.name = common.PodsAllocatable
		query = `kube_node_status_allocatable_pods{}`
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getNodeMetric)
	}
	nodeWorkloadWriters.AddMetricWorkloadWriters(common.CpuLimits, common.CpuRequests, common.MemoryLimits, common.MemoryRequests)

	mh.name = common.Limits
	query = `sum(kube_pod_container_resource_limits{}) by (node, resource)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getNodeMetric)
	if common.Found(indicators, mh.name, false) {
		mh.name = common.CpuLimit
		query = `sum(kube_pod_container_resource_limits_cpu_cores{}) by (node)*1000`
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getNodeMetric)
		mh.name = common.MemLimit
		query = `sum(kube_pod_container_resource_limits_memory_bytes{}) by (node)/1024/1024`
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getNodeMetric)
	}

	mh.name = common.Requests
	query = `sum(kube_pod_container_resource_requests{}) by (node,resource)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getNodeMetric)
	if common.Found(indicators, mh.name, false) {
		mh.name = common.CpuRequest
		query = `sum(kube_pod_container_resource_requests_cpu_cores{}) by (node)*1000`
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getNodeMetric)
		mh.name = common.MemRequest
		query = `sum(kube_pod_container_resource_requests_memory_bytes{}) by (node)/1024/1024`
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getNodeMetric)
	}

	nodeWorkloadWriters.CloseAndClearWorkloadWriters(common.NodeEntityKind)

	writeConfig()
	writeAttributes()

	// get the reservation percent metrics
	var rpCoreMetrics = []string{"kube_pod_container_resource_requests", "kube_node_status_allocatable"}
	wmhs := []*common.WorkloadMetricHolder{common.CpuReservationPercent, common.MemoryReservationPercent}
	var rpFormats = map[bool]string{
		true:  `%s{resource="%s"}`,
		false: `%s_%s{}`,
	}
	var rpArgs = map[bool][]string{
		true:  {"cpu", "memory"},
		false: {"cpu_cores", "memory_bytes"},
	}

	qw := simpleQueryWrapper(common.Node)
	for _, f := range common.FoundIndicatorCounter(indicators, common.Requests) {
		q := make([]string, len(rpCoreMetrics))
		for i, wmh := range wmhs {
			for j, rpcm := range rpCoreMetrics {
				q[j] = qw.SumQuery.Wrap(fmt.Sprintf(rpFormats[f], rpcm, rpArgs[f][i]))
			}
			query = fmt.Sprintf(`(%s / %s) * 100`, q[0], q[1])
			wmh.GetWorkload(query, qw.MetricField, common.NodeEntityKind)
		}
	}

	query = qw.CountQuery.Wrap(`kube_pod_info{}`)
	common.PodCount.GetWorkload(query, qw.MetricField, common.NodeEntityKind)

	// bail out if detected that Prometheus Node Exporter metrics are not present for any cluster
	if !HasNodeExporter(range5Min) {
		err = fmt.Errorf("prometheus node exporter metrics not present for any cluster")
		common.LogError(err, "entity=%s", common.NodeEntityKind)
		return
	}

	for _, qw = range GetQueryWrappers(&queryWrappers, queryWrappersMap) {

		query = fmt.Sprintf(`sum(irate(node_cpu_seconds_total{mode!="idle"}[%sm])) by (%s) / on (%s) group_left count(node_cpu_seconds_total{mode="idle"}) by (%s) *100`, common.Params.Collection.SampleRateSt, qw.MetricField[0], qw.MetricField[0], qw.MetricField[0])
		query = qw.Query.Wrap(query)
		common.CpuUtilization.GetWorkload(query, qw.MetricField, common.NodeEntityKind)

		query = qw.Query.Wrap(`node_memory_MemTotal_bytes{} - node_memory_MemFree_bytes{}`)
		common.MemoryBytes.GetWorkload(query, qw.MetricField, common.NodeEntityKind)

		query = qw.Query.Wrap(`node_memory_MemTotal_bytes{} - (node_memory_MemFree_bytes{} + node_memory_Cached_bytes{} + node_memory_Buffers_bytes{})`)
		common.MemoryActualWorkload.GetWorkload(query, qw.MetricField, common.NodeEntityKind)

		query = qw.Query.Wrap(`round(increase(node_vmstat_oom_kill{}[` + common.Params.Collection.SampleRateSt + `m]))`)
		common.OomKillEvents.GetWorkload(query, qw.MetricField, common.NodeEntityKind)

		query = qw.SumQuery.Wrap(`round(increase(node_cpu_core_throttles_total{}[` + common.Params.Collection.SampleRateSt + `m]))`)
		common.CpuThrottlingEvents.GetWorkload(query, qw.MetricField, common.NodeEntityKind)

		query = qw.SumQuery.Wrap(`irate(node_disk_read_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.DiskReadBytes.GetWorkload(query, qw.MetricField, common.NodeEntityKind)

		query = qw.SumQuery.Wrap(`irate(node_disk_written_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.DiskWriteBytes.GetWorkload(query, qw.MetricField, common.NodeEntityKind)

		query = qw.SumQuery.Wrap(`irate(node_disk_read_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_disk_written_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.DiskTotalBytes.GetWorkload(query, qw.MetricField, common.NodeEntityKind)

		query = qw.SumQuery.Wrap(`irate(node_disk_reads_completed_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.DiskReadOps.GetWorkload(query, qw.MetricField, common.NodeEntityKind)

		query = qw.SumQuery.Wrap(`irate(node_disk_writes_completed_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.DiskWriteOps.GetWorkload(query, qw.MetricField, common.NodeEntityKind)

		query = qw.SumQuery.Wrap(`(irate(node_disk_reads_completed_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_disk_writes_completed_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m]))`)
		common.DiskTotalOps.GetWorkload(query, qw.MetricField, common.NodeEntityKind)

		query = qw.SumQuery.Wrap(`irate(node_network_receive_bytes_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.NetReceivedBytes.GetWorkload(query, qw.MetricField, common.NodeEntityKind)

		query = qw.SumQuery.Wrap(`irate(node_network_transmit_bytes_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.NetSentBytes.GetWorkload(query, qw.MetricField, common.NodeEntityKind)

		query = qw.SumQuery.Wrap(`irate(node_network_transmit_bytes_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_network_receive_bytes_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.NetTotalBytes.GetWorkload(query, qw.MetricField, common.NodeEntityKind)

		query = qw.SumQuery.Wrap(`irate(node_network_receive_packets_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.NetReceivedPackets.GetWorkload(query, qw.MetricField, common.NodeEntityKind)

		query = qw.SumQuery.Wrap(`irate(node_network_transmit_packets_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.NetSentPackets.GetWorkload(query, qw.MetricField, common.NodeEntityKind)

		query = qw.SumQuery.Wrap(`irate(node_network_transmit_packets_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_network_receive_packets_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.NetTotalPackets.GetWorkload(query, qw.MetricField, common.NodeEntityKind)

	}
}

const (
	labelProviderId       = "provider_id"
	labelKubeletVersion   = "kubelet_version"
	labelOS               = "label_kubernetes_io_os"
	labelOSBeta           = "label_beta_kubernetes_io_os"
	labelInstanceType     = "label_node_kubernetes_io_instance_type"
	labelInstanceTypeBeta = "label_beta_kubernetes_io_instance_type"
	labelArch             = "label_kubernetes_io_arch"
	labelArchBeta         = "label_beta_kubernetes_io_arch"
	labelRegion           = "label_topology_kubernetes_io_region"
	labelRegionBeta       = "label_failure_domain_beta_kubernetes_io_region"
	labelZone             = "label_topology_kubernetes_io_zone"
	labelZoneBeta         = "label_failure_domain_beta_kubernetes_io_zone"
)

var osLabels = []string{labelOS, labelOSBeta}
var instanceTypeLabels = []string{labelInstanceType, labelInstanceTypeBeta}
var archLabels = []string{labelArch, labelArchBeta}
var regionLabels = []string{labelRegion, labelRegionBeta}
var zoneLabels = []string{labelZone, labelZoneBeta}

// GetOSInstanceType is exported for usage in nodegroup package as well
func GetOSInstanceType(labelMap map[string]string) (string, string) {
	opSys := getLabel(labelMap, osLabels)
	instanceType := getLabel(labelMap, instanceTypeLabels)
	return opSys, instanceType
}

func getArchRegionZone(labelMap map[string]string) (string, string, string) {
	arch := getLabel(labelMap, archLabels)
	region := getLabel(labelMap, regionLabels)
	zone := getLabel(labelMap, zoneLabels)
	return arch, region, zone
}

func getLabel(labelMap map[string]string, candidateLabelNames []string) (value string) {
	for _, labelName := range candidateLabelNames {
		if val, ok := labelMap[labelName]; ok {
			value = val
			break
		}
	}
	return
}

func createNode(cluster string, result model.Matrix) {
	var f bool
	if _, f = nodes[cluster]; !f {
		if l := result.Len(); l > 0 {
			nodes[cluster] = make(map[string]*node, l)
		}
	}
	for _, ss := range result {
		nodeName := string(ss.Metric[common.Node])
		// The provider Id is optional, populated for cloud providers k8s clusters (EKS, AKS, GKE)
		// and OpenShift clusters on VMWare / cloud infrastructure. It's usually not populated for
		// bare-metal / kind / OpenShift CRC trial clusters.
		// common.AddToLabelMap() truncates the labels at 255 characters, and we have observed
		// Azure's AKS provider Ids longer than 225 characters (the length depends on resource groups and VMSS names etc.).
		// So not taking the risk here and getting the provider Id directly from the metric (rather than
		// later from the labelMap).
		provId := string(ss.Metric[labelProviderId])
		var k8sVer string
		// if we did not get the node k8s version, get it from the kubelet version
		if k8sVer = kubernetes.GetNodeVersion(cluster, nodeName); k8sVer == common.Empty {
			k8sVer = string(ss.Metric[labelKubeletVersion])
		}
		var n *node
		if n, f = nodes[cluster][nodeName]; !f {
			n = &node{
				name:                        nodeName,
				providerId:                  provId,
				k8sVersion:                  k8sVer,
				labelMap:                    make(map[string]string),
				netSpeedBytes:               common.UnknownValue,
				cpuCapacity:                 common.UnknownValue,
				memCapacity:                 common.UnknownValue,
				ephemeralStorageCapacity:    common.UnknownValue,
				podsCapacity:                common.UnknownValue,
				hugepages2MiCapacity:        common.UnknownValue,
				cpuAllocatable:              common.UnknownValue,
				memAllocatable:              common.UnknownValue,
				ephemeralStorageAllocatable: common.UnknownValue,
				podsAllocatable:             common.UnknownValue,
				hugepages2MiAllocatable:     common.UnknownValue,
				cpuLimit:                    common.UnknownValue,
				cpuRequest:                  common.UnknownValue,
				memLimit:                    common.UnknownValue,
				memRequest:                  common.UnknownValue,
			}
			nodes[cluster][nodeName] = n
		}
		// collect all the labels too
		for key, value := range ss.Metric {
			common.AddToLabelMap(string(key), string(value), n.labelMap)
		}
	}
}

const (
	nodeExporterPivotQuery = "max(node_cpu_seconds_total{}) by (node, instance)"
	HasNodeLabel           = "node_label_node_name"     // "node" label is present and has the node name
	HasInstanceLabelPodIp  = "instance_label_pod_ip"    // "node" label is absent, "instance" label has a format of IP address:port
	HasInstanceLabelOther  = "instance_label_node_name" // "node" label is absent, "instance" label has a different format and assumed to be node name
)

var once sync.Once

func DetermineNodeExporter(range5Min *v1.Range) {
	once.Do(func() {
		_, _ = common.CollectAndProcessMetric(nodeExporterPivotQuery, range5Min, determineNodeExporter)
	})
}

var nodeExporterIndicators = make(map[string][]string)

func determineNodeExporter(cluster string, result model.Matrix) {
	if l := result.Len(); l > 0 {
		ss := result[l-1]
		var indicator string
		var f bool
		if _, f = ss.Metric[common.Node]; f {
			indicator = HasNodeLabel
		} else {
			var instance model.LabelValue
			if instance, f = ss.Metric[common.Instance]; f {
				if _, _, err := nnet.ParseAddress(string(instance)); err == nil {
					indicator = HasInstanceLabelPodIp
				} else {
					indicator = HasInstanceLabelOther
				}
			}
		}
		if f {
			nodeExporterIndicators[indicator] = append(nodeExporterIndicators[indicator], cluster)
		}
	}
}

// HasNodeExporter returns true if node exporter metrics are present for any cluster
func HasNodeExporter(range5Min *v1.Range) bool {
	DetermineNodeExporter(range5Min)
	return len(nodeExporterIndicators) > 0
}

var queryWrapperKeys = []string{HasNodeLabel, HasInstanceLabelPodIp, HasInstanceLabelOther}

type QueryWrapper struct {
	Query, SumQuery, CountQuery *common.WorkloadQueryWrapper
	MetricField                 []model.LabelName
}

const (
	ByPodIpSuffix     = `, "pod_ip", "$1", "instance", "(.*):.*")) by (pod_ip) * on (pod_ip) group_right kube_pod_info{pod=~".*node-exporter.*"}`
	byPodIpSuffixNode = ByPodIpSuffix + `) by (node)`
)

var queryWrappersMap = map[string]*QueryWrapper{
	HasInstanceLabelPodIp: {
		Query: &common.WorkloadQueryWrapper{
			Prefix: "max(max(label_replace(",
			Suffix: byPodIpSuffixNode,
		},
		SumQuery: &common.WorkloadQueryWrapper{
			Prefix: "max(sum(label_replace(",
			Suffix: byPodIpSuffixNode,
		},
		MetricField: []model.LabelName{common.Node},
	},
	HasNodeLabel:          simpleQueryWrapper(common.Node),
	HasInstanceLabelOther: simpleQueryWrapper(common.Instance),
}

func simpleQueryWrapper(labelName string) *QueryWrapper {
	sfx := fmt.Sprintf(") by (%s)", labelName)
	return &QueryWrapper{
		Query: &common.WorkloadQueryWrapper{},
		SumQuery: &common.WorkloadQueryWrapper{
			Prefix: "sum(",
			Suffix: sfx,
		},
		CountQuery: &common.WorkloadQueryWrapper{
			Prefix: "count(",
			Suffix: sfx,
		},
		MetricField: []model.LabelName{model.LabelName(labelName)},
	}
}

var queryWrappers []*QueryWrapper

// GetQueryWrappers returns the query wrappers that are relevant for the current environment
func GetQueryWrappers(qws *[]*QueryWrapper, qwm map[string]*QueryWrapper) []*QueryWrapper {
	if qws == nil {
		return nil
	}
	if *qws == nil {
		for _, key := range queryWrapperKeys {
			if _, f := nodeExporterIndicators[key]; f {
				var qw *QueryWrapper
				if qw, f = qwm[key]; f {
					*qws = append(*qws, qw)
				}
			}
		}
	}
	return *qws
}
