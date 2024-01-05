package node

import (
	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/prometheus/common/model"
)

var indicators = make(map[string]int)

func getNode(cluster string, ss *model.SampleStream, nodeLabel model.LabelName) (ng *node, f bool) {
	var nodeLabelValue model.LabelValue
	nodeLabelValue, f = ss.Metric[nodeLabel]
	if f {
		nodeValue := string(nodeLabelValue)
		ng, f = nodes[cluster][nodeValue]
	}
	return
}

const (
	ephemeralStorage = "ephemeral_storage"
	hugePages        = "hugepages_2Mi"
	nodeExporter     = "node_exporter"
)

func setValue(what *int, value float64) {
	*what = int(value)
}

type metricHolder struct {
	name string
}

func getIndicator(s string) string {
	switch s {
	case common.NetSpeedBytes:
		return nodeExporter
	default:
		return s
	}
}

func (mh *metricHolder) getNodeMetric(cluster string, result model.Matrix) {
	if result.Len() > 0 {
		switch mh.name {
		case common.Capacity, common.Allocatable, common.NetSpeedBytes, common.Limits, common.Requests:
			ind := getIndicator(mh.name)
			indicators[ind] = indicators[ind] + 1
		default:
		}
	}
	for _, ss := range result {
		n, ok := getNode(cluster, ss, common.Node)
		if !ok {
			continue
		}
		value := common.LastValue(ss)
		res := ss.Metric[common.Resource]
		switch mh.name {
		case common.Capacity:
			switch res {
			case common.Cpu:
				setValue(&n.cpuCapacity, value)
			case common.Memory:
				setValue(&n.memCapacity, value)
			case model.LabelValue(common.Pods):
				setValue(&n.podsCapacity, value)
			case ephemeralStorage:
				setValue(&n.ephemeralStorageCapacity, value)
			case hugePages:
				setValue(&n.hugepages2MiCapacity, value)
			}
		case common.Allocatable:
			switch res {
			case common.Cpu:
				setValue(&n.cpuAllocatable, value)
			case common.Memory:
				setValue(&n.memAllocatable, value)
			case model.LabelValue(common.Pods):
				setValue(&n.podsAllocatable, value)
			case ephemeralStorage:
				setValue(&n.ephemeralStorageAllocatable, value)
			case hugePages:
				setValue(&n.hugepages2MiAllocatable, value)
			}
		case common.CpuCapacity:
			setValue(&n.cpuCapacity, value)
		case common.MemCapacity:
			setValue(&n.memCapacity, value)
		case common.PodsCapacity:
			setValue(&n.podsCapacity, value)
		case common.CpuAllocatable:
			setValue(&n.cpuAllocatable, value)
		case common.MemAllocatable:
			setValue(&n.memAllocatable, value)
		case common.PodsAllocatable:
			setValue(&n.podsAllocatable, value)
		case common.NetSpeedBytes:
			setValue(&n.netSpeedBytes, value)
		case common.Limits:
			switch res {
			case common.Cpu:
				setValue(&n.cpuLimit, common.MCores(value))
			case common.Memory:
				setValue(&n.memLimit, common.MiB(value))
			}
		case common.Requests:
			switch res {
			case common.Cpu:
				setValue(&n.cpuRequest, common.MCores(value))
			case common.Memory:
				setValue(&n.memRequest, common.MiB(value))
			}
		case common.CpuLimit:
			setValue(&n.cpuLimit, value)
		case common.CpuRequest:
			setValue(&n.cpuRequest, value)
		case common.MemLimit:
			setValue(&n.memLimit, value)
		case common.MemRequest:
			setValue(&n.memRequest, value)
		}
	}
}

// getNodeMetricString is used to parse the label-based results from Prometheus related to nodes and store them
func getNodeMetricString(cluster string, result model.Matrix) {
	for _, ss := range result {
		node, ok := getNode(cluster, ss, common.Node)
		if !ok {
			continue
		}
		for key, value := range ss.Metric {
			common.AddToLabelMap(string(key), string(value), node.labelMap)
		}
	}
}
