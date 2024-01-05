package cluster

import (
	"fmt"
	"github.com/densify-dev/container-data-collection/internal/common"
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

func (cmh *clusterMetricHolder) getClusterMetric(clusterName string, result model.Matrix) {
	var cl *cluster
	if l := result.Len(); l > 0 {
		var f bool
		if cl, f = clusters[clusterName]; !f {
			cl = &cluster{
				cpuLimit:   common.UnknownValue,
				cpuRequest: common.UnknownValue,
				memLimit:   common.UnknownValue,
				memRequest: common.UnknownValue,
			}
			clusters[clusterName] = cl
		}
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

	if _, err = fmt.Fprintln(attributeWrite, "Name,VirtualTechnology,VirtualDomain,CpuLimit,CpuRequest,MemoryLimit,MemoryRequest"); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.ClusterEntityKind)
		return
	}
	if _, err = fmt.Fprintf(attributeWrite, "%s,Clusters,%s", name, name); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.ClusterEntityKind)
		return
	}
	values := []int{cl.cpuLimit, cl.cpuRequest, cl.memLimit, cl.memRequest}
	last := len(values) - 1
	for i, value := range values {
		if err = common.PrintCSVIntValue(attributeWrite, value, i == last); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.ClusterEntityKind)
			return
		}
	}
}

func Metrics() {
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

	common.GetConditionalMetricsWorkload(indicators, common.Requests, map[string][]model.LabelName{common.Empty: nil}, common.ClusterEntityKind)

	query = `avg(sum(irate(node_cpu_seconds_total{mode!="idle"}[` + common.Params.Collection.SampleRateSt + `m])) by (instance) / on (instance) group_left count(node_cpu_seconds_total{mode="idle"}) by (instance) *100)`
	common.CpuUtilization.GetWorkload(query, nil, common.ClusterEntityKind)

	query = `avg(node_memory_MemTotal_bytes{} - node_memory_MemFree_bytes{})`
	common.MemoryBytes.GetWorkload(query, nil, common.ClusterEntityKind)

	query = `avg(node_memory_MemTotal_bytes{} - (node_memory_MemFree_bytes{} + node_memory_Cached_bytes{} + node_memory_Buffers_bytes{}))`
	common.MemoryActualWorkload.GetWorkload(query, nil, common.ClusterEntityKind)

	query = `avg(sum(irate(node_disk_read_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])) by (instance))`
	common.DiskReadBytes.GetWorkload(query, nil, common.ClusterEntityKind)

	query = `avg(sum(irate(node_disk_written_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])) by (instance))`
	common.DiskWriteBytes.GetWorkload(query, nil, common.ClusterEntityKind)

	query = `avg(sum(irate(node_disk_read_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_disk_written_bytes_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])) by (instance))`
	common.DiskTotalBytes.GetWorkload(query, nil, common.ClusterEntityKind)

	query = `avg(sum(irate(node_disk_read_time_seconds_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m]) / irate(node_disk_io_time_seconds_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])) by (instance))`
	common.DiskReadOps.GetWorkload(query, nil, common.ClusterEntityKind)

	query = `avg(sum(irate(node_disk_write_time_seconds_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m]) / irate(node_disk_io_time_seconds_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])) by (instance))`
	common.DiskWriteOps.GetWorkload(query, nil, common.ClusterEntityKind)

	query = `avg(sum((irate(node_disk_read_time_seconds_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_disk_write_time_seconds_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])) / irate(node_disk_io_time_seconds_total{device!~"dm-.*"}[` + common.Params.Collection.SampleRateSt + `m])) by (instance))`
	common.DiskTotalOps.GetWorkload(query, nil, common.ClusterEntityKind)

	query = `avg(sum(irate(node_network_receive_bytes_total{device!~"veth.*"}[` + common.Params.Collection.SampleRateSt + `m])) by (instance))`
	common.NetReceivedBytes.GetWorkload(query, nil, common.ClusterEntityKind)

	query = `avg(sum(irate(node_network_transmit_bytes_total{device!~"veth.*"}[` + common.Params.Collection.SampleRateSt + `m])) by (instance))`
	common.NetSentBytes.GetWorkload(query, nil, common.ClusterEntityKind)

	query = `avg(sum(irate(node_network_transmit_bytes_total{device!~"veth.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_network_receive_bytes_total{device!~"veth.*"}[` + common.Params.Collection.SampleRateSt + `m])) by (instance))`
	common.NetTotalBytes.GetWorkload(query, nil, common.ClusterEntityKind)

	query = `avg(sum(irate(node_network_receive_packets_total{device!~"veth.*"}[` + common.Params.Collection.SampleRateSt + `m])) by (instance))`
	common.NetReceivedPackets.GetWorkload(query, nil, common.ClusterEntityKind)

	query = `avg(sum(irate(node_network_transmit_packets_total{device!~"veth.*"}[` + common.Params.Collection.SampleRateSt + `m])) by (instance))`
	common.NetSentPackets.GetWorkload(query, nil, common.ClusterEntityKind)

	query = `avg(sum(irate(node_network_transmit_packets_total{device!~"veth.*"}[` + common.Params.Collection.SampleRateSt + `m]) + irate(node_network_receive_packets_total{device!~"veth.*"}[` + common.Params.Collection.SampleRateSt + `m])) by (instance))`
	common.NetTotalPackets.GetWorkload(query, nil, common.ClusterEntityKind)
}
