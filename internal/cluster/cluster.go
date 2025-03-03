package cluster

import (
	"fmt"
	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/densify-dev/container-data-collection/internal/kubernetes"
	"github.com/densify-dev/container-data-collection/internal/node"
	"github.com/prometheus/common/model"
	"os"
)

type cluster struct {
	cpuLimit, cpuRequest, memLimit, memRequest int
}

var clusters = make(map[string]*cluster)

var indicators = make(map[string]int)

type clusterMetricHolder struct {
	metric string
}

func createCluster(name string) {
	if _, f := clusters[name]; !f {
		clusters[name] = &cluster{
			cpuLimit:   common.UnknownValue,
			cpuRequest: common.UnknownValue,
			memLimit:   common.UnknownValue,
			memRequest: common.UnknownValue,
		}
	}
}

func (cmh *clusterMetricHolder) getClusterMetric(clusterName string, result model.Matrix) {
	var cl *cluster
	var f bool
	if cl, f = clusters[clusterName]; !f {
		return
	}
	if l := result.Len(); l > 0 {
		indicators[cmh.metric] = indicators[cmh.metric] + 1
	}
	for _, ss := range result {
		value := common.LastValue(ss)
		switch cmh.metric {
		case common.Limits:
			switch ss.Metric[common.Resource] {
			case common.Memory:
				cl.memLimit = common.IntMiB(value)
			case common.Cpu:
				cl.cpuLimit = common.IntMCores(value)
			}
		case common.Requests:
			switch ss.Metric[common.Resource] {
			case common.Memory:
				cl.memRequest = common.IntMiB(value)
			case common.Cpu:
				cl.cpuRequest = common.IntMCores(value)
			}
		case common.CpuLimit:
			cl.cpuLimit = int(value)
		case common.CpuRequest:
			cl.cpuRequest = int(value)
		case common.MemLimit:
			cl.memLimit = int(value)
		case common.MemRequest:
			cl.memRequest = int(value)
		}
	}
}

func writeConfig() {
	for name := range clusters {
		writeConf(name)
	}
}

func writeConf(name string) {
	configWrite, err := os.Create(common.GetFileNameByType(name, common.ClusterEntityKind, common.Config))
	if err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.ClusterEntityKind)
		return
	}

	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.ClusterEntityKind)
		}
	}(configWrite)

	if _, err = fmt.Fprintln(configWrite, "AuditTime,Name"); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.ClusterEntityKind)
		return
	}

	if _, err = fmt.Fprintf(configWrite, "%s,%s\n", common.FormatCurrentTime(), name); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.ClusterEntityKind)
		return
	}
}

func writeAttributes() {
	for name, cl := range clusters {
		writeAttrs(name, cl)
	}
}

func writeAttrs(name string, cl *cluster) {
	attributeWrite, err := os.Create(common.GetFileNameByType(name, common.ClusterEntityKind, common.Attributes))
	if err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.ClusterEntityKind)
		return
	}
	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.ClusterEntityKind)
		}
	}(attributeWrite)

	if _, err = fmt.Fprintln(attributeWrite, "Name,VirtualTechnology,VirtualDomain,CpuLimit,CpuRequest,MemoryLimit,MemoryRequest,K8sVersion"); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.ClusterEntityKind)
		return
	}
	if _, err = fmt.Fprintf(attributeWrite, "%s,Clusters,%s", name, name); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.ClusterEntityKind)
		return
	}
	values := []int{cl.cpuLimit, cl.cpuRequest, cl.memLimit, cl.memRequest}
	for _, value := range values {
		if err = common.PrintCSVNumberValue(attributeWrite, value, false); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.ClusterEntityKind)
			return
		}
	}
	if err = common.PrintCSVStringValue(attributeWrite, kubernetes.GetClusterVersion(name), true); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.ClusterEntityKind)
		return
	}
}

var queryWrappersMap = map[string]*node.QueryWrapper{
	node.HasNodeLabel: {
		Query:       &common.WorkloadQueryWrapper{},
		SumQuery:    &common.WorkloadQueryWrapper{},
		MetricField: []model.LabelName{common.Node},
	},
	node.HasInstanceLabelPodIp: {
		Query:       &common.WorkloadQueryWrapper{},
		SumQuery:    &common.WorkloadQueryWrapper{},
		MetricField: []model.LabelName{common.Instance},
	},
	node.HasInstanceLabelOther: {
		Query:       &common.WorkloadQueryWrapper{},
		SumQuery:    &common.WorkloadQueryWrapper{},
		MetricField: []model.LabelName{common.Instance},
	},
}

var queryWrappers []*node.QueryWrapper

func Metrics() {
	for _, clusterName := range common.ClusterNames {
		createCluster(clusterName)
	}
	var query string
	range5Min := common.TimeRange()
	cmh := &clusterMetricHolder{metric: common.Limits}
	query = `sum(kube_pod_container_resource_limits{}) by (resource)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, cmh.getClusterMetric)
	if common.Found(indicators, common.Limits, false) {
		cmh.metric = common.CpuLimit
		query = `sum(kube_pod_container_resource_limits_cpu_cores{}*1000)`
		_, _ = common.CollectAndProcessMetric(query, range5Min, cmh.getClusterMetric)
		cmh.metric = common.MemLimit
		query = `sum(kube_pod_container_resource_limits_memory_bytes{}/1024/1024)`
		_, _ = common.CollectAndProcessMetric(query, range5Min, cmh.getClusterMetric)
	}
	cmh.metric = common.Requests
	query = `sum(kube_pod_container_resource_requests{}) by (resource)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, cmh.getClusterMetric)
	if common.Found(indicators, common.Requests, false) {
		cmh.metric = common.CpuRequest
		query = `sum(kube_pod_container_resource_requests_cpu_cores{}*1000)`
		_, _ = common.CollectAndProcessMetric(query, range5Min, cmh.getClusterMetric)
		cmh.metric = common.MemRequest
		query = `sum(kube_pod_container_resource_requests_memory_bytes{}/1024/1024)`
		_, _ = common.CollectAndProcessMetric(query, range5Min, cmh.getClusterMetric)
	}
	writeConfig()
	writeAttributes()

	common.GetConditionalMetricsWorkload(indicators, common.Requests, map[string][]model.LabelName{common.Empty: nil}, common.ClusterEntityKind, common.Metric)

	// bail out if detected that Prometheus Node Exporter metrics are not present for any cluster
	if !node.HasNodeExporter(range5Min) {
		err := fmt.Errorf("prometheus node exporter metrics not present for any cluster")
		common.LogError(err, "entity=%s", common.ClusterEntityKind)
		return
	}

	for _, qw := range node.GetQueryWrappers(&queryWrappers, queryWrappersMap) {

		query = fmtQuery(`avg(sum(irate(node_cpu_seconds_total{mode!="idle"}[%sm])) by (%s) / on (%s) group_left count(node_cpu_seconds_total{mode="idle"}) by (%s) *100)`, qw, 1, 3)
		common.CpuUtilization.GetWorkload(query, nil, common.ClusterEntityKind)

		query = `sum(node_memory_MemTotal_bytes{} - node_memory_MemFree_bytes{})`
		common.MemoryBytes.GetWorkload(query, nil, common.ClusterEntityKind)

		query = `sum(node_memory_MemTotal_bytes{} - (node_memory_MemFree_bytes{} + node_memory_Cached_bytes{} + node_memory_Buffers_bytes{} + node_memory_SReclaimable_bytes{}))`
		common.MemoryActualWorkload.GetWorkload(query, nil, common.ClusterEntityKind)

		query = fmtQuery(`sum(sum(irate(node_disk_read_bytes_total{device!~"dm-.*"}[%sm])) by (%s))`, qw, 1, 1)
		common.DiskReadBytes.GetWorkload(query, nil, common.ClusterEntityKind)

		query = fmtQuery(`sum(sum(irate(node_disk_written_bytes_total{device!~"dm-.*"}[%sm])) by (%s))`, qw, 1, 1)
		common.DiskWriteBytes.GetWorkload(query, nil, common.ClusterEntityKind)

		query = fmtQuery(`sum(sum(irate(node_disk_read_bytes_total{device!~"dm-.*"}[%sm]) + irate(node_disk_written_bytes_total{device!~"dm-.*"}[%sm])) by (%s))`, qw, 2, 1)
		common.DiskTotalBytes.GetWorkload(query, nil, common.ClusterEntityKind)

		query = fmtQuery(`sum(sum(irate(node_disk_reads_completed_total{device!~"dm-.*"}[%sm])) by (%s))`, qw, 1, 1)
		common.DiskReadOps.GetWorkload(query, nil, common.ClusterEntityKind)

		query = fmtQuery(`sum(sum(irate(node_disk_writes_completed_total{device!~"dm-.*"}[%sm])) by (%s))`, qw, 1, 1)
		common.DiskWriteOps.GetWorkload(query, nil, common.ClusterEntityKind)

		query = fmtQuery(`sum(sum((irate(node_disk_reads_completed_total{device!~"dm-.*"}[%sm]) + irate(node_disk_writes_completed_total{device!~"dm-.*"}[%sm]))) by (%s))`, qw, 2, 1)
		common.DiskTotalOps.GetWorkload(query, nil, common.ClusterEntityKind)

		query = fmtQuery(`sum(sum(irate(node_network_receive_bytes_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[%sm])) by (%s))`, qw, 1, 1)
		common.NetReceivedBytes.GetWorkload(query, nil, common.ClusterEntityKind)

		query = fmtQuery(`sum(sum(irate(node_network_transmit_bytes_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[%sm])) by (%s))`, qw, 1, 1)
		common.NetSentBytes.GetWorkload(query, nil, common.ClusterEntityKind)

		query = fmtQuery(`sum(sum(irate(node_network_transmit_bytes_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[%sm]) + irate(node_network_receive_bytes_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[%sm])) by (%s))`, qw, 2, 1)
		common.NetTotalBytes.GetWorkload(query, nil, common.ClusterEntityKind)

		query = fmtQuery(`sum(sum(irate(node_network_receive_packets_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[%sm])) by (%s))`, qw, 1, 1)
		common.NetReceivedPackets.GetWorkload(query, nil, common.ClusterEntityKind)

		query = fmtQuery(`sum(sum(irate(node_network_transmit_packets_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[%sm])) by (%s))`, qw, 1, 1)
		common.NetSentPackets.GetWorkload(query, nil, common.ClusterEntityKind)

		query = fmtQuery(`sum(sum(irate(node_network_transmit_packets_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[%sm]) + irate(node_network_receive_packets_total{device!~"veth.*|docker.*|cilium.*|lxc.*"}[%sm])) by (%s))`, qw, 2, 1)
		common.NetTotalPackets.GetWorkload(query, nil, common.ClusterEntityKind)
	}
}

func fmtQuery(queryFmt string, qw *node.QueryWrapper, numSampleRate, numLabel int) string {
	s := make([]any, numSampleRate+numLabel)
	for i := 0; i < numSampleRate; i++ {
		s[i] = common.Params.Collection.SampleRateSt
	}
	for i := 0; i < numLabel; i++ {
		s[numSampleRate+i] = qw.MetricField[0]
	}
	return fmt.Sprintf(queryFmt, s...)
}
