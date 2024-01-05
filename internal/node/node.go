package node

import (
	"fmt"
	"github.com/densify-dev/container-data-collection/internal/common"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"sync"
)

// A node structure. Used for storing attributes and config details.
type node struct {
	labelMap                                                                                              map[string]string
	netSpeedBytes, cpuCapacity, memCapacity, ephemeralStorageCapacity, podsCapacity, hugepages2MiCapacity int
	cpuAllocatable, memAllocatable, ephemeralStorageAllocatable, podsAllocatable, hugepages2MiAllocatable int
	cpuLimit, cpuRequest, memLimit, memRequest                                                            int
}

// Map that labels and values will be stored in
var nodes = make(map[string]map[string]*node)

type queryWrapper struct {
	query, sumQuery *common.WorkloadQueryWrapper
	metricField     []model.LabelName
}

const (
	ByPodIpSuffix     = `, "pod_ip", "$1", "instance", "(.*):.*")) by (pod_ip) * on (pod_ip) group_right kube_pod_info{pod=~".*node-exporter.*"}`
	byPodIpSuffixNode = ByPodIpSuffix + `) by (node)`
)

var queryWrapperByPodIp = map[bool]*queryWrapper{
	true: {
		query: &common.WorkloadQueryWrapper{
			Prefix: "max(max(label_replace(",
			Suffix: byPodIpSuffixNode,
		},
		sumQuery: &common.WorkloadQueryWrapper{
			Prefix: "max(sum(label_replace(",
			Suffix: byPodIpSuffixNode,
		},
		metricField: []model.LabelName{common.Node},
	},
	false: {
		query: &common.WorkloadQueryWrapper{},
		sumQuery: &common.WorkloadQueryWrapper{
			Prefix: "sum(",
			Suffix: ") by (instance)",
		},
		metricField: []model.LabelName{common.Instance},
	},
}

// Metrics a global func for collecting node level metrics in prometheus
func Metrics() {
	var query string
	var err error
	range5Min := common.TimeRange()

	//Query and store kubernetes node information/labels
	query = "max(kube_node_labels{}) by (instance, node)"
	if _, err := common.CollectAndProcessMetric(query, range5Min, createNode); err != nil {
		// error already handled
		return
	}

	// Additional config/attribute queries
	query = `kube_node_labels{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, getNodeMetricString)

	query = `kube_node_info{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, getNodeMetricString)

	query = `kube_node_role{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, getNodeMetricString)

	mh := &metricHolder{name: common.NetSpeedBytes}

	// netSpeedBytes is also used to determine if node exporter is present
	query = `label_replace(node_network_speed_bytes{}, "pod_ip", "$1", "instance", "(.*):.*")`
	_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getNodeMetric)

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

	mh.name = common.Limits
	query = `sum(kube_pod_container_resource_limits{}) by (node, resource)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getNodeMetric)
	if common.Found(indicators, mh.name, false) {
		mh.name = common.CpuLimit
		query = `sum(kube_pod_container_resource_limits_cpu_cores{}) by (node)*1000`
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getNodeMetric)
		mh.name = common.MemLimit
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

	writeConfig()
	writeAttributes()

	// bail out if detected that Prometheus Node Exporter is not present
	if !common.Found(indicators, nodeExporter, true) {
		err = fmt.Errorf("prometheus node exporter not present in any cluster")
		common.LogError(err, "entity=%s", common.NodeEntityKind)
		return
	}

	DetermineByPodIp(range5Min)

	for _, f := range FoundCountersByPodIp() {
		qw := queryWrapperByPodIp[f]

		query = qw.query.Wrap(`sum(irate(node_cpu_seconds_total{mode!="idle"}[` + common.Params.Collection.SampleRateSt + `m])) by (instance) / on (instance) group_left count(node_cpu_seconds_total{mode="idle"}) by (instance) *100`)
		common.CpuUtilization.GetWorkload(query, qw.metricField, common.NodeEntityKind)

		query = qw.query.Wrap(`node_memory_MemTotal_bytes{} - node_memory_MemFree_bytes{}`)
		common.MemoryBytes.GetWorkload(query, qw.metricField, common.NodeEntityKind)

		query = qw.query.Wrap(`node_memory_MemTotal_bytes{} - (node_memory_MemFree_bytes{} + node_memory_Cached_bytes{} + node_memory_Buffers_bytes{})`)
		common.MemoryActualWorkload.GetWorkload(query, qw.metricField, common.NodeEntityKind)

		query = qw.sumQuery.Wrap(`irate(node_disk_read_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.DiskReadBytes.GetWorkload(query, qw.metricField, common.NodeEntityKind)

		query = qw.sumQuery.Wrap(`irate(node_disk_written_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.DiskWriteBytes.GetWorkload(query, qw.metricField, common.NodeEntityKind)

		query = qw.sumQuery.Wrap(`irate(node_disk_read_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_disk_written_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.DiskTotalBytes.GetWorkload(query, qw.metricField, common.NodeEntityKind)

		query = qw.sumQuery.Wrap(`irate(node_disk_read_time_seconds_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m]) / irate(node_disk_io_time_seconds_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.DiskReadOps.GetWorkload(query, qw.metricField, common.NodeEntityKind)

		query = qw.sumQuery.Wrap(`irate(node_disk_write_time_seconds_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m]) / irate(node_disk_io_time_seconds_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.DiskWriteOps.GetWorkload(query, qw.metricField, common.NodeEntityKind)

		query = qw.sumQuery.Wrap(`(irate(node_disk_read_time_seconds_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_disk_write_time_seconds_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])) / irate(node_disk_io_time_seconds_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.DiskTotalOps.GetWorkload(query, qw.metricField, common.NodeEntityKind)

		query = qw.sumQuery.Wrap(`irate(node_network_receive_bytes_total{device!~"veth.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.NetReceivedBytes.GetWorkload(query, qw.metricField, common.NodeEntityKind)

		query = qw.sumQuery.Wrap(`irate(node_network_transmit_bytes_total{device!~"veth.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.NetSentBytes.GetWorkload(query, qw.metricField, common.NodeEntityKind)

		query = qw.sumQuery.Wrap(`irate(node_network_transmit_bytes_total{device!~"veth.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_network_receive_bytes_total{device!~"veth.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.NetTotalBytes.GetWorkload(query, qw.metricField, common.NodeEntityKind)

		query = qw.sumQuery.Wrap(`irate(node_network_receive_packets_total{device!~"veth.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.NetReceivedPackets.GetWorkload(query, qw.metricField, common.NodeEntityKind)

		query = qw.sumQuery.Wrap(`irate(node_network_transmit_packets_total{device!~"veth.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.NetSentPackets.GetWorkload(query, qw.metricField, common.NodeEntityKind)

		query = qw.sumQuery.Wrap(`irate(node_network_transmit_packets_total{device!~"veth.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_network_receive_packets_total{device!~"veth.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		common.NetTotalPackets.GetWorkload(query, qw.metricField, common.NodeEntityKind)
	}
}

const (
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
		if _, f = nodes[cluster][nodeName]; !f {
			nodes[cluster][nodeName] = &node{
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
				labelMap:                    make(map[string]string),
			}
		}
	}
}

const (
	byPodIpQueryFormat = `max(max(label_replace(sum(irate(node_cpu_seconds_total{mode!="idle"}[%sm])) by (instance) / on (instance) group_left count(node_cpu_seconds_total{mode="idle"}) by (instance) *100, "pod_ip", "$1", "instance", "(.*):.*")) by (pod_ip) * on (pod_ip) group_right kube_pod_info{pod=~".*node-exporter.*"}) by (node)`
)

var byPodIp int

func incByPodIp(_ string, result model.Matrix) {
	if result.Len() > 0 {
		byPodIp++
	}
}

var once sync.Once

// DetermineByPodIp checks to see if instance is IP address that need to link to pod to get name or if instance = node name
func DetermineByPodIp(range5Min v1.Range) {
	once.Do(func() {
		query := fmt.Sprintf(byPodIpQueryFormat, common.Params.Collection.SampleRateSt)
		_, _ = common.CollectAndProcessMetric(query, range5Min, incByPodIp)
	})
}

func FoundCountersByPodIp() []bool {
	return common.FoundCounter(byPodIp)
}
