package node

import (
	"fmt"
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
)

func setValue(what *int, value float64) {
	*what = int(value)
}

type metricHolder struct {
	name      string
	labelName model.LabelName
}

func (mh *metricHolder) getNodeMetric(cluster string, result model.Matrix) {
	if result.Len() > 0 {
		switch mh.name {
		case common.Capacity, common.Allocatable, common.Limits, common.Requests:
			indicators[mh.name] = indicators[mh.name] + 1
		default:
		}
	}
	for _, ss := range result {
		n, ok := getNode(cluster, ss, mh.labelName)
		if !ok {
			continue
		}
		value := common.LastValue(ss)
		res := ss.Metric[common.Resource]
		nwp := &nodeWorkloadProducer{cluster: cluster, node: n}
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
				common.WriteWorkload(nwp, nodeWorkloadWriters, common.CpuLimits, ss, common.MCores[float64])
			case common.Memory:
				setValue(&n.memLimit, common.MiB(value))
				common.WriteWorkload(nwp, nodeWorkloadWriters, common.MemoryLimits, ss, common.MiB[float64])
			}
		case common.Requests:
			switch res {
			case common.Cpu:
				setValue(&n.cpuRequest, common.MCores(value))
				common.WriteWorkload(nwp, nodeWorkloadWriters, common.CpuRequests, ss, common.MCores[float64])
			case common.Memory:
				setValue(&n.memRequest, common.MiB(value))
				common.WriteWorkload(nwp, nodeWorkloadWriters, common.MemoryRequests, ss, common.MiB[float64])
			}
		case common.CpuLimit:
			setValue(&n.cpuLimit, value)
			common.WriteWorkload(nwp, nodeWorkloadWriters, common.CpuLimits, ss, nil)
		case common.CpuRequest:
			setValue(&n.cpuRequest, value)
			common.WriteWorkload(nwp, nodeWorkloadWriters, common.CpuRequests, ss, nil)
		case common.MemLimit:
			setValue(&n.memLimit, value)
			common.WriteWorkload(nwp, nodeWorkloadWriters, common.MemoryLimits, ss, nil)
		case common.MemRequest:
			setValue(&n.memRequest, value)
			common.WriteWorkload(nwp, nodeWorkloadWriters, common.MemoryRequests, ss, nil)
		}
	}
}

// getNodeMetricString is used to parse the label-based results from Prometheus related to nodes and store them
func getNodeMetricString(cluster string, result model.Matrix) {
	for _, ss := range result {
		n, ok := getNode(cluster, ss, common.Node)
		if !ok {
			continue
		}
		for key, value := range ss.Metric {
			common.AddToLabelMap(string(key), string(value), n.labelMap)
		}
	}
}

func getNodeTaints(cluster string, result model.Matrix) {
	for _, ss := range result {
		n, ok := getNode(cluster, ss, common.Node)
		if !ok {
			continue
		}
		t := &taint{
			key:    string(ss.Metric[common.Key]),
			value:  string(ss.Metric[common.Value]),
			effect: string(ss.Metric[common.Effect]),
		}
		n.taints = append(n.taints, t)
	}
}

type nodeWorkloadProducer struct {
	cluster string
	node    *node
}

func (nwp *nodeWorkloadProducer) GetCluster() string {
	return nwp.cluster
}

func (nwp *nodeWorkloadProducer) GetEntityKind() string {
	return common.NodeEntityKind
}

func (nwp *nodeWorkloadProducer) GetRowPrefixes() []string {
	return []string{fmt.Sprintf("%s,%s", nwp.cluster, overrideNodeName(nwp.cluster, nwp.node.name))}
}

func (nwp *nodeWorkloadProducer) ShouldWrite(_ string) bool {
	return true
}
