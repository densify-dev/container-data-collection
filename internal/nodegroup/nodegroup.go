package nodegroup

import (
	"fmt"
	"os"
	"strings"

	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/densify-dev/container-data-collection/internal/node"
	"github.com/prometheus/common/model"
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
		if _, err = fmt.Fprintf(configWrite, "%s,%s,%s", common.FormatCurrentTime(), name, AdjustNodeGroupName(name, nodeGroupName)); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeGroupEntityKind)
			return
		}
		values := []int{ng.cpuCapacity, ng.cpuCapacity, 1, 1, ng.memCapacity}
		for _, value := range values {
			if err = common.PrintCSVNumberValue(configWrite, value, false); err != nil {
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
		if _, err = fmt.Fprintf(attributeWrite, "%s,%s,NodeGroup,%s", name, AdjustNodeGroupName(name, nodeGroupName), name); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeGroupEntityKind)
			return
		}
		values := []int{ng.cpuLimit, ng.cpuRequest, ng.memLimit, ng.memRequest, ng.currentSize}
		for _, value := range values {
			if err = common.PrintCSVNumberValue(attributeWrite, value, false); err != nil {
				common.LogError(err, common.DefaultLogFormat, name, common.NodeGroupEntityKind)
				return
			}
		}
		nodes := node.OverrideNodeNames(name, ng.nodes, common.Or)
		if err = common.PrintCSVStringValue(attributeWrite, nodes, false); err != nil {
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

// overrideNodeGroupNameFieldsFunc assumes that the LAST field is the node group name
func overrideNodeGroupNameFieldsFunc(cluster string, fields []string) ([]string, bool) {
	l := len(fields)
	ok := l >= 1
	if ok {
		fields[l-1] = AdjustNodeGroupName(cluster, fields[l-1])
	}
	return fields, ok
}

// getWorkload used to query for the workload data and then calls write workload
func getWorkload(wmh *common.WorkloadMetricHolder, query string, labelNames []model.LabelName, ccqa common.QueryAdjuster) {
	m := getQueryToMetricField(query, labelNames, ccqa)
	qps := make(map[string]*common.QueryProcessor, len(m))
	for q, mf := range m {
		qps[q] = &common.QueryProcessor{MetricFields: mf, FF: overrideNodeGroupNameFieldsFunc}
	}
	wmh.GetWorkloadQueryVariants(2, qps, common.NodeGroupEntityKind)
}

func getQueryToMetricField(query string, labelNames []model.LabelName, ccqa common.QueryAdjuster) map[string][]model.LabelName {
	m := make(map[string][]model.LabelName, len(labelNames))
	for _, metricField := range labelNames {
		q := generateQuery(query, metricField, ccqa)
		f := []model.LabelName{metricField}
		m[q] = f
	}
	return m
}

func (ngh *nodeGroupHolder) createNodeGroup(cluster string, result model.Matrix) {
	if _, f := ensureLabelFeature(cluster); f {
		createNodeGroup(cluster, result, ngh.nodeGroupLabel)
		getNodeGroupMetricString(cluster, result, ngh.nodeGroupLabel)
	}
}

func createNodeGroup(cluster string, result model.Matrix, labelName model.LabelName) {
	if _, f := nodeGroups[cluster]; !f {
		if l := result.Len(); l > 0 {
			nodeGroups[cluster] = make(map[string]*nodeGroup, l)
		}
	}
	for _, ss := range result {
		var nodeGroupName, nodeName string
		var name model.LabelValue
		var f bool
		if name, f = ss.Metric[labelName]; !f {
			continue
		} else {
			nodeGroupName = string(name)
		}
		var ng *nodeGroup
		if ng, f = nodeGroups[cluster][nodeGroupName]; !f {
			ng = &nodeGroup{
				// currentSize is initialized to 0!
				cpuLimit:    common.UnknownValue,
				cpuRequest:  common.UnknownValue,
				cpuCapacity: common.UnknownValue,
				memLimit:    common.UnknownValue,
				memRequest:  common.UnknownValue,
				memCapacity: common.UnknownValue,
				labelMap:    make(map[string]string),
			}
			nodeGroups[cluster][nodeGroupName] = ng
		}
		if name, f = ss.Metric[common.Node]; !f {
			continue
		} else {
			nodeName = string(name)
		}
		if !strings.Contains(ng.nodes, nodeName) {
			ng.nodes = ng.nodes + nodeName + common.Or
			ng.currentSize++
		}
	}

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

func generateQuery(queryFmt string, labelName model.LabelName, ccqa common.QueryAdjuster, extraArgs ...any) string {
	return ccqa(common.FormatRepeatedAuto(queryFmt, labelName, extraArgs...))
}

// Metrics a global func for collecting node level metrics in prometheus
func Metrics() {
	var query string
	var err error
	range5Min := common.TimeRange()

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

	if err = determineLabelFeatures(range5Min); err != nil {
		common.LogErrorWithLevel(1, common.Error, err, "entity=%s", common.NodeGroupEntityKind)
	}

	if len(clusterFeatures) < common.NumClusters() {
		if err = determineOpenshiftFeatures(range5Min); err != nil {
			common.LogErrorWithLevel(1, common.Error, err, "entity=%s", common.NodeGroupEntityKind)
		}
	}

	if len(clusterFeatures) < common.NumClusters() {
		if err = determineRoleFeatures(range5Min); err != nil {
			common.LogErrorWithLevel(1, common.Error, err, "entity=%s", common.NodeGroupEntityKind)
		}
	}

	if len(clusterFeatures) < common.NumClusters() {
		// TODO: set default
	}

	common.RegisterClusterQueryExclusion(common.ExcComment, common.ExcludeQueryByClusterComment)

	ccqas := common.GetClusterCommentQueryAdapters()

	for cluster, ccqa := range ccqas {
		var queryFmt string
		cf := clusterFeatures[cluster]
		for _, labelName := range cf.LabelNames() {
			configSuffix := QuerySuffixFmt(cf, common.ConfigSt, true)
			ngh := &nodeGroupHolder{nodeGroupLabel: labelName}
			ngmh := &nodeGroupMetricHolder{nodeGroupHolder: ngh}
			for _, qualifier := range qualifiers {
				for _, f := range common.FoundIndicatorCounter(foundUnified, qualifier) {
					for res, coreQuery := range resourceCoreQueries[qualifier][f] {
						queryFmt = fmt.Sprintf("sum(sum(%s%s) by (node)%s", coreQuery, operands[res], configSuffix)
						query = generateQuery(queryFmt, labelName, ccqa)
						ngmh.metric = common.DromedaryCase(res, qualifier)
						_, _ = common.CollectAndProcessMetric(query, range5Min, ngmh.getNodeGroupMetric)
					}
				}
			}
			configSuffix = QuerySuffixFmt(cf, common.ConfigSt, true, common.Resource)
			queryFmt = fmt.Sprintf("sum(kube_node_status_capacity{}%s", configSuffix)
			query = generateQuery(queryFmt, labelName, ccqa, common.Resource)
			ngmh.metric = common.Capacity
			_, _ = common.CollectAndProcessMetric(query, range5Min, ngmh.getNodeGroupMetric)
		}
		writeAttributes()
		writeConfig()

		nodeGroupLabels := cf.LabelNames()
		var queryWrappers []*node.QueryWrapper

		qmf := getQueryToMetricField(QuerySuffixFmt(cf, common.Metric, false), nodeGroupLabels, ccqa)
		common.GetConditionalMetricsWorkload(foundUnified, common.Request, qmf, common.NodeGroupEntityKind, common.Metric)

		query = CountQueryFmt(cf)
		getWorkload(common.CurrentSize, query, nodeGroupLabels, ccqa)

		queryWrappersMap := QueryWrappersMap(cf)
		qws := node.GetQueryWrappers(&queryWrappers, queryWrappersMap)

		if node.HasNodeExporter(range5Min) {
			for _, qw := range qws {
				query = fmt.Sprintf(`sum(irate(node_cpu_seconds_total{mode!="idle"}[%sm])) by (%s) / on (%s) group_left count(node_cpu_seconds_total{mode="idle"}) by (%s) *100`, common.Params.Collection.SampleRateSt, qw.MetricField[0], qw.MetricField[0], qw.MetricField[0])
				query = qw.Query.GenerateWrapper(node.SumToAverage, nil).Wrap(query)
				getWorkload(common.CpuUtilization, query, nodeGroupLabels, ccqa)

				query = qw.Query.Wrap(`(node_memory_MemTotal_bytes{} - node_memory_MemFree_bytes{})`)
				getWorkload(common.MemoryBytes, query, nodeGroupLabels, ccqa)

				query = qw.Query.Wrap(fmt.Sprintf("(%s)", node.GetMemActualQuery()))
				getWorkload(common.MemoryActualWorkload, query, nodeGroupLabels, ccqa)

				query = qw.SumQuery.Wrap(`irate(node_disk_read_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
				getWorkload(common.DiskReadBytes, query, nodeGroupLabels, ccqa)

				query = qw.SumQuery.Wrap(`irate(node_disk_written_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
				getWorkload(common.DiskWriteBytes, query, nodeGroupLabels, ccqa)

				query = qw.SumQuery.Wrap(`irate(node_disk_read_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_disk_written_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
				getWorkload(common.DiskTotalBytes, query, nodeGroupLabels, ccqa)

				query = qw.SumQuery.Wrap(`irate(node_disk_reads_completed_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
				getWorkload(common.DiskReadOps, query, nodeGroupLabels, ccqa)

				query = qw.SumQuery.Wrap(`irate(node_disk_writes_completed_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
				getWorkload(common.DiskWriteOps, query, nodeGroupLabels, ccqa)

				query = qw.SumQuery.Wrap(`(irate(node_disk_reads_completed_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_disk_writes_completed_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m]))`)
				getWorkload(common.DiskTotalOps, query, nodeGroupLabels, ccqa)

				query = qw.SumQuery.Wrap(`irate(node_network_receive_bytes_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
				getWorkload(common.NetReceivedBytes, query, nodeGroupLabels, ccqa)

				query = qw.SumQuery.Wrap(`irate(node_network_transmit_bytes_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
				getWorkload(common.NetSentBytes, query, nodeGroupLabels, ccqa)

				query = qw.SumQuery.Wrap(`irate(node_network_transmit_bytes_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_network_receive_bytes_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
				getWorkload(common.NetTotalBytes, query, nodeGroupLabels, ccqa)

				query = qw.SumQuery.Wrap(`irate(node_network_receive_packets_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
				getWorkload(common.NetReceivedPackets, query, nodeGroupLabels, ccqa)

				query = qw.SumQuery.Wrap(`irate(node_network_transmit_packets_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
				getWorkload(common.NetSentPackets, query, nodeGroupLabels, ccqa)

				query = qw.SumQuery.Wrap(`irate(node_network_transmit_packets_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_network_receive_packets_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[` + common.Params.Collection.SampleRateSt + `m])`)
				getWorkload(common.NetTotalPackets, query, nodeGroupLabels, ccqa)
			}
		} else {
			common.LogAll(1, common.Error, "entity=%s prometheus node exporter metrics not present for any cluster", common.NodeGroupEntityKind)
		}

		if node.HasDcgmExporter(range5Min) {
			gwmhs := map[string]*common.WorkloadMetricHolder{
				common.Empty:               common.GpuUtilizationAvg,
				node.GpuPercentQuerySuffix: common.GpuUtilizationGpusAvg,
			}
			// All DCGM queries are transformed using label_replace to have the node label
			qw := queryWrappersMap[node.HasNodeLabel]
			for q, wmh := range gwmhs {
				query = fmt.Sprintf("avg(%s) by (%s)", common.SafeDcgmGpuUtilizationQuery+q, qw.MetricField[0])
				query = qw.Query.GenerateWrapper(node.SumToAverage, nil).Wrap(query)
				getWorkload(wmh, query, nodeGroupLabels, ccqa)
			}
			query = fmt.Sprintf(" 100 * avg(%s) by (%s)", common.DcgmExporterLabelReplace("DCGM_FI_DEV_FB_USED{} / (DCGM_FI_DEV_FB_USED{} + DCGM_FI_DEV_FB_FREE{})"), qw.MetricField[0])
			query = qw.Query.GenerateWrapper(node.SumToAverage, nil).Wrap(query)
			getWorkload(common.GpuMemUtilizationAvg, query, nodeGroupLabels, ccqa)

			query = qw.SumQuery.Wrap(common.DcgmExporterLabelReplace("DCGM_FI_DEV_FB_USED{}"))
			getWorkload(common.GpuMemUsedAvg, query, nodeGroupLabels, ccqa)

			query = qw.SumQuery.Wrap(common.DcgmExporterLabelReplace("DCGM_FI_DEV_POWER_USAGE{}"))
			getWorkload(common.GpuPowerUsageAvg, query, nodeGroupLabels, ccqa)
		} else {
			common.LogAll(1, common.Info, "entity=%s Nvidia DCGM exporter metrics not present for any cluster", common.NodeGroupEntityKind)
		}
	}
	common.UnregisterClusterQueryExclusion(common.ExcComment)
}
