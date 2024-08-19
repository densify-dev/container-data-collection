package nodegroup

import (
	"fmt"
	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/densify-dev/container-data-collection/internal/node"
	"github.com/prometheus/common/model"
	"os"
	"strings"
)

type nodeGroup struct {
	nodes                                                                             string
	cpuLimit, cpuRequest, cpuCapacity, memLimit, memRequest, memCapacity, currentSize int
	labelMap                                                                          map[string]string
}

var nodeGroups = make(map[string]map[string]*nodeGroup)

func getNodeGroup(cluster string, ss *model.SampleStream, nodeGroupLabel model.LabelName) (ng *nodeGroup, f bool) {
	var nodeGroupLabelValue model.LabelValue
	nodeGroupLabelValue, f = ss.Metric[nodeGroupLabel]
	if f {
		nodeGroupValue := string(nodeGroupLabelValue)
		ng, f = nodeGroups[cluster][nodeGroupValue]
	}
	return
}

func getNodeGroupMetricString(cluster string, result model.Matrix, nodeGroupLabel model.LabelName) {
	for _, ss := range result {
		ng, ok := getNodeGroup(cluster, ss, nodeGroupLabel)
		if !ok {
			continue
		}
		for key, value := range ss.Metric {
			common.AddToLabelMap(string(key), string(value), ng.labelMap)
		}
	}
}

type nodeGroupHolder struct {
	nodeGroupLabel model.LabelName
}

type nodeGroupMetricHolder struct {
	*nodeGroupHolder
	metric string
}

func (ngmh *nodeGroupMetricHolder) getNodeGroupMetric(cluster string, result model.Matrix) {
	for _, ss := range result {
		nodeGroup, ok := getNodeGroup(cluster, ss, ngmh.nodeGroupLabel)
		if !ok {
			continue
		}
		value := int(common.LastValue(ss))

		switch ngmh.metric {
		case common.CpuLimit:
			nodeGroup.cpuLimit = value
		case common.CpuRequest:
			nodeGroup.cpuRequest = value
		case common.CpuCapacity:
			nodeGroup.cpuCapacity = value
		case common.MemLimit:
			nodeGroup.memLimit = value
		case common.MemRequest:
			nodeGroup.memRequest = value
		case common.MemCapacity:
			nodeGroup.memCapacity = value
		case common.Capacity:
			switch ss.Metric[common.Resource] {
			case common.Cpu:
				nodeGroup.cpuCapacity = value
			case common.Memory:
				nodeGroup.memCapacity = value
			}
		}
	}
}

func writeConfig() {
	for name, cluster := range nodeGroups {
		writeConf(name, cluster)
	}
}

func writeConf(name string, cluster map[string]*nodeGroup) {
	configWrite, err := os.Create(common.GetFileNameByType(name, common.NodeGroupEntityKind, common.Config))
	if err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.NodeGroupEntityKind)
		return
	}

	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeGroupEntityKind)
		}
	}(configWrite)

	if _, err = fmt.Fprintln(configWrite, "AuditTime,ClusterName,NodeGroupName,HwTotalCpus,HwTotalPhysicalCpus,HwCoresPerCpu,HwThreadsPerCore,HwTotalMemory,HwModel,OsName"); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.NodeGroupEntityKind)
		return
	}

	for nodeGroupName, ng := range cluster {
		if _, err = fmt.Fprintf(configWrite, "%s,%s,%s", common.FormatCurrentTime(), name, nodeGroupName); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeGroupEntityKind)
			return
		}
		values := []int{ng.cpuCapacity, ng.cpuCapacity, 1, 1, ng.memCapacity}
		for _, value := range values {
			if err = common.PrintCSVIntValue(configWrite, value, false); err != nil {
				common.LogError(err, common.DefaultLogFormat, name, common.NodeGroupEntityKind)
				return
			}
		}
		opSys, instanceType := node.GetOSInstanceType(ng.labelMap)
		if _, err = fmt.Fprintf(configWrite, ",%s,%s\n", instanceType, opSys); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeGroupEntityKind)
			return
		}
	}
}

func writeAttributes() {
	for name, cluster := range nodeGroups {
		writeAttrs(name, cluster)
	}
}

func writeAttrs(name string, cluster map[string]*nodeGroup) {
	attributeWrite, err := os.Create(common.GetFileNameByType(name, common.NodeGroupEntityKind, common.Attributes))
	if err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.NodeGroupEntityKind)
		return
	}

	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeGroupEntityKind)
		}
	}(attributeWrite)

	if _, err = fmt.Fprintln(attributeWrite, "ClusterName,NodeGroupName,VirtualTechnology,VirtualDomain,CpuLimit,CpuRequest,MemoryLimit,MemoryRequest,CurrentSize,CurrentNodes,NodeLabels"); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.NodeGroupEntityKind)
		return
	}
	for nodeGroupName, ng := range cluster {
		if _, err = fmt.Fprintf(attributeWrite, "%s,%s,NodeGroup,%s", name, nodeGroupName, name); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeGroupEntityKind)
			return
		}
		values := []int{ng.cpuLimit, ng.cpuRequest, ng.memLimit, ng.memRequest, ng.currentSize}
		for _, value := range values {
			if err = common.PrintCSVIntValue(attributeWrite, value, false); err != nil {
				common.LogError(err, common.DefaultLogFormat, name, common.NodeGroupEntityKind)
				return
			}
		}
		if err = common.PrintCSVJoinedStringValue(attributeWrite, ng.nodes, common.Or, false); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeGroupEntityKind)
			return
		}
		if err = common.PrintCSVStringValue(attributeWrite, common.Empty, false); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeGroupEntityKind)
			return
		}
		if err = common.PrintCSVLabelMap(attributeWrite, ng.labelMap, true); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeGroupEntityKind)
			return
		}
	}
}

const (
	nodeGroupLabelPlaceholder = "nodeGroupLabelPlaceholder"
)

// getWorkload used to query for the workload data and then calls write workload
func getWorkload(wmh *common.WorkloadMetricHolder, query string, nodeGroupLabelNames []model.LabelName) {
	m := getQueryToMetricField(query, nodeGroupLabelNames)
	qps := make(map[string]*common.QueryProcessor, len(m))
	for q, mf := range m {
		qps[q] = &common.QueryProcessor{MetricFields: mf}
	}
	wmh.GetWorkloadQueryVariants(2, qps, common.NodeGroupEntityKind)
}

func getQueryToMetricField(query string, nodeGroupLabelNames []model.LabelName) map[string][]model.LabelName {
	m := make(map[string][]model.LabelName, len(nodeGroupLabelNames))
	for _, metricField := range nodeGroupLabelNames {
		q := strings.ReplaceAll(query, nodeGroupLabelPlaceholder, string(metricField))
		f := []model.LabelName{metricField}
		m[q] = f
	}
	return m
}

var nodeGroupLabels = make(map[model.LabelName]bool)
var ngl []model.LabelName

func detectNameLabel(_ string, result model.Matrix) {
	// ignoring the cluster, we only collect the label names
	for _, ss := range result {
		for labelName := range ss.Metric {
			nodeGroupLabels[labelName] = true
		}
	}
}

func (ngh *nodeGroupHolder) createNodeGroup(cluster string, result model.Matrix) {
	var f bool
	if _, f = nodeGroups[cluster]; !f {
		if l := result.Len(); l > 0 {
			nodeGroups[cluster] = make(map[string]*nodeGroup, l)
		}
	}
	for _, ss := range result {
		nodeGroupName := string(ss.Metric[ngh.nodeGroupLabel])
		nodeName := string(ss.Metric[common.Node])
		if _, f = nodeGroups[cluster][nodeGroupName]; !f {
			nodeGroups[cluster][nodeGroupName] = &nodeGroup{
				// currentSize is initialized to 0!
				cpuLimit:    common.UnknownValue,
				cpuRequest:  common.UnknownValue,
				cpuCapacity: common.UnknownValue,
				memLimit:    common.UnknownValue,
				memRequest:  common.UnknownValue,
				memCapacity: common.UnknownValue,
				labelMap:    make(map[string]string),
			}
		}
		nodeGroups[cluster][nodeGroupName].nodes = nodeGroups[cluster][nodeGroupName].nodes + nodeName + common.Or
		nodeGroups[cluster][nodeGroupName].currentSize++
	}
	getNodeGroupMetricString(cluster, result, ngh.nodeGroupLabel)
}

var foundUnified = make(map[string]int)

func incUnifiedLimits(_ string, result model.Matrix) {
	incUnified(result, common.Limit)
}

func incUnifiedRequests(_ string, result model.Matrix) {
	incUnified(result, common.Request)
}

func incUnified(result model.Matrix, qualifier string) {
	if result.Len() > 0 {
		foundUnified[qualifier] = foundUnified[qualifier] + 1
	}
}

var qualifiers = []string{common.Limit, common.Request}

var resourceCoreQueries = map[string]map[bool]map[string]string{
	common.Limit: {
		true: {
			common.Cpu:    `kube_pod_container_resource_limits{resource="cpu"}`,
			common.Memory: `kube_pod_container_resource_limits{resource="memory"}`},
		false: {
			common.Cpu:    `kube_pod_container_resource_limits_cpu_cores{}`,
			common.Memory: `kube_pod_container_resource_limits_memory_bytes{}`},
	},
	common.Request: {
		true: {
			common.Cpu:    `kube_pod_container_resource_requests{resource="cpu"}`,
			common.Memory: `kube_pod_container_resource_requests{resource="memory"}`},
		false: {
			common.Cpu:    `kube_pod_container_resource_requests_cpu_cores{}`,
			common.Memory: `kube_pod_container_resource_requests_memory_bytes{}`},
	},
}

var operands = map[string]string{common.Cpu: "*1000", common.Memory: "/1024/1024"}

const (
	workloadNodeGroupSuffixNoBracket = ` * on (node) group_right kube_node_labels{` + nodeGroupLabelPlaceholder + `=~".+"}) by (` + nodeGroupLabelPlaceholder
	workloadNodeGroupSuffix          = workloadNodeGroupSuffixNoBracket + ")"
	byPodIpSuffix                    = node.ByPodIpSuffix + workloadNodeGroupSuffix
)

var queryWrappersMap = map[string]*node.QueryWrapper{
	node.HasInstanceLabelPodIp: {
		Query: &common.WorkloadQueryWrapper{
			Prefix: "avg(max(label_replace(",
			Suffix: byPodIpSuffix,
		},
		SumQuery: &common.WorkloadQueryWrapper{
			Prefix: "avg(sum(label_replace(",
			Suffix: byPodIpSuffix,
		},
		MetricField: []model.LabelName{common.Node},
	},
	node.HasNodeLabel: {
		Query: &common.WorkloadQueryWrapper{
			Prefix: "avg(",
			Suffix: workloadNodeGroupSuffix,
		},
		SumQuery: &common.WorkloadQueryWrapper{
			Prefix: "avg(sum(",
			Suffix: `) by (node)` + workloadNodeGroupSuffix,
		},
		MetricField: []model.LabelName{common.Node},
	},
	node.HasInstanceLabelOther: {
		Query: &common.WorkloadQueryWrapper{
			Prefix: "avg(label_replace(",
			Suffix: `, "node", "$1", "instance", "(.*)")` + workloadNodeGroupSuffix,
		},
		SumQuery: &common.WorkloadQueryWrapper{
			Prefix: "avg(label_replace(sum(",
			Suffix: `) by (instance), "node", "$1", "instance", "(.*)")` + workloadNodeGroupSuffix,
		},
		MetricField: []model.LabelName{common.Instance},
	},
}

var queryWrappers []*node.QueryWrapper

// Metrics a global func for collecting node level metrics in prometheus
func Metrics() {
	var query string
	var err error
	range5Min := common.TimeRange()

	query = `avg(kube_node_labels{}) by (` + common.ToPrometheusLabelNameList(common.Params.Collection.NodeGroupList) + `)`
	// even if there are no labels, we'll get one stream with value 1 and empty label set,
	// so need to check for length of nodeGroupLabels i.s.o. the number returned by CollectAndProcessMetric
	if _, err = common.CollectAndProcessMetric(query, range5Min, detectNameLabel); err != nil || len(nodeGroupLabels) == 0 {
		// error already handled
		return
	}
	// we need node group labels in many places as a slice
	ngl = common.KeySet(nodeGroupLabels)

	query = `sum(kube_pod_container_resource_limits{}) by (node, resource)`
	if _, err = common.CollectAndProcessMetric(query, range5Min, incUnifiedLimits); err != nil {
		// error already handled
		return
	}

	query = `sum(kube_pod_container_resource_requests{}) by (node, resource)`
	if _, err = common.CollectAndProcessMetric(query, range5Min, incUnifiedRequests); err != nil {
		// error already handled
		return
	}

	var nodeGroupSuffix string

	for ng := range nodeGroupLabels {
		ngh := &nodeGroupHolder{nodeGroupLabel: ng}
		ngStr := string(ng)
		query = `kube_node_labels{` + ngStr + `=~".+"}`
		if _, err = common.CollectAndProcessMetric(query, range5Min, ngh.createNodeGroup); err != nil {
			// error already handled
			continue
		}
		ngmh := &nodeGroupMetricHolder{nodeGroupHolder: ngh}
		nodeGroupSuffix = ` * on (node) group_left (` + ngStr + `) kube_node_labels{` + ngStr + `=~".+"}) by (` + ngStr + `)`
		for _, qualifier := range qualifiers {
			for _, f := range common.FoundIndicatorCounter(foundUnified, qualifier) {
				for res, coreQuery := range resourceCoreQueries[qualifier][f] {
					query = fmt.Sprintf("avg(sum(%s%s) by (node)%s", coreQuery, operands[res], nodeGroupSuffix)
					ngmh.metric = common.DromedaryCase(res, qualifier)
					_, _ = common.CollectAndProcessMetric(query, range5Min, ngmh.getNodeGroupMetric)
				}
			}
		}
		query = `avg(kube_node_status_capacity{} * on (node) group_left (` + ngStr + `) kube_node_labels{` + ngStr + `=~".+"}) by (` + ngStr + `,resource)`
		ngmh.metric = common.Capacity
		var n int
		if n, err = common.CollectAndProcessMetric(query, range5Min, ngmh.getNodeGroupMetric); err != nil || n < common.NumClusters() {
			query = `avg(kube_node_status_capacity_cpu_cores{}` + nodeGroupSuffix
			ngmh.metric = common.CpuCapacity
			_, _ = common.CollectAndProcessMetric(query, range5Min, ngmh.getNodeGroupMetric)
			query = `avg(kube_node_status_capacity_memory_bytes{}` + operands[common.Memory] + nodeGroupSuffix
			ngmh.metric = common.MemCapacity
			_, _ = common.CollectAndProcessMetric(query, range5Min, ngmh.getNodeGroupMetric)
		}
	}
	writeAttributes()
	writeConfig()

	qmf := getQueryToMetricField(workloadNodeGroupSuffixNoBracket, ngl)
	common.GetConditionalMetricsWorkload(foundUnified, common.Request, qmf, common.NodeGroupEntityKind, common.Metric)

	query = `sum(kube_node_labels{` + nodeGroupLabelPlaceholder + `=~".+"}) by (` + nodeGroupLabelPlaceholder + `)`
	getWorkload(common.CurrentSize, query, ngl)

	// bail out if detected that Prometheus Node Exporter metrics are not present for any cluster
	if !node.HasNodeExporter(range5Min) {
		err = fmt.Errorf("prometheus node exporter metrics not present for any cluster")
		common.LogError(err, "entity=%s", common.NodeGroupEntityKind)
		return
	}

	for _, qw := range node.GetQueryWrappers(&queryWrappers, queryWrappersMap) {

		query = fmt.Sprintf(`sum(irate(node_cpu_seconds_total{mode!="idle"}[%sm])) by (%s) / on (%s) group_left count(node_cpu_seconds_total{mode="idle"}) by (%s) *100`, common.Params.Collection.SampleRateSt, qw.MetricField[0], qw.MetricField[0], qw.MetricField[0])
		query = qw.Query.Wrap(query)
		getWorkload(common.CpuUtilization, query, ngl)

		query = qw.Query.Wrap(`(node_memory_MemTotal_bytes{} - node_memory_MemFree_bytes{})`)
		getWorkload(common.MemoryBytes, query, ngl)

		query = qw.Query.Wrap(`(node_memory_MemTotal_bytes{} - (node_memory_MemFree_bytes{} + node_memory_Cached_bytes{} + node_memory_Buffers_bytes{}))`)
		getWorkload(common.MemoryActualWorkload, query, ngl)

		query = qw.SumQuery.Wrap(`irate(node_disk_read_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		getWorkload(common.DiskReadBytes, query, ngl)

		query = qw.SumQuery.Wrap(`irate(node_disk_written_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		getWorkload(common.DiskWriteBytes, query, ngl)

		query = qw.SumQuery.Wrap(`irate(node_disk_read_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_disk_written_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		getWorkload(common.DiskTotalBytes, query, ngl)

		query = qw.SumQuery.Wrap(`irate(node_disk_reads_completed_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		getWorkload(common.DiskReadOps, query, ngl)

		query = qw.SumQuery.Wrap(`irate(node_disk_writes_completed_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		getWorkload(common.DiskWriteOps, query, ngl)

		query = qw.SumQuery.Wrap(`(irate(node_disk_reads_completed_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_disk_writes_completed_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m]))`)
		getWorkload(common.DiskTotalOps, query, ngl)

		query = qw.SumQuery.Wrap(`irate(node_network_receive_bytes_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		getWorkload(common.NetReceivedBytes, query, ngl)

		query = qw.SumQuery.Wrap(`irate(node_network_transmit_bytes_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		getWorkload(common.NetSentBytes, query, ngl)

		query = qw.SumQuery.Wrap(`irate(node_network_transmit_bytes_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_network_receive_bytes_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		getWorkload(common.NetTotalBytes, query, ngl)

		query = qw.SumQuery.Wrap(`irate(node_network_receive_packets_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		getWorkload(common.NetReceivedPackets, query, ngl)

		query = qw.SumQuery.Wrap(`irate(node_network_transmit_packets_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		getWorkload(common.NetSentPackets, query, ngl)

		query = qw.SumQuery.Wrap(`irate(node_network_transmit_packets_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_network_receive_packets_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
		getWorkload(common.NetTotalPackets, query, ngl)
	}
}
