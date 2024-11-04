package node

import (
	"fmt"
	"github.com/densify-dev/container-data-collection/internal/common"
	"os"
)

func writeConfig() {
	for name, cluster := range nodes {
		writeConf(name, cluster)
	}
}

// writeConf will create the config.csv file that will be sent to Densify by the Forwarder.
func writeConf(name string, cluster map[string]*node) {
	configWrite, err := os.Create(common.GetFileNameByType(name, common.NodeEntityKind, common.Config))
	if err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.NodeEntityKind)
		return
	}

	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeEntityKind)
		}
	}(configWrite)

	if _, err = fmt.Fprintln(configWrite, "AuditTime,ClusterName,NodeName,HwModel,OsName,HwTotalCpus,HwTotalPhysicalCpus,HwCoresPerCpu,HwThreadsPerCore,HwTotalMemory,HwMaxNetworkIoBps"); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.NodeEntityKind)
		return
	}

	for nodeName, n := range cluster {
		opSys, instanceType := GetOSInstanceType(n.labelMap)
		if _, err = fmt.Fprintf(configWrite, "%s,%s,%s,%s,%s", common.FormatCurrentTime(), name, nodeName, instanceType, opSys); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeEntityKind)
			return
		}
		memCap := common.UnknownValue
		if n.memCapacity != common.UnknownValue {
			memCap = n.memCapacity / 1024 / 1024
		}
		values := []int{n.cpuCapacity, n.cpuCapacity, 1, 1, memCap, n.netSpeedBytes}
		last := len(values) - 1
		for i, value := range values {
			if err = common.PrintCSVIntValue(configWrite, value, i == last); err != nil {
				common.LogError(err, common.DefaultLogFormat, name, common.NodeEntityKind)
				return
			}
		}
	}
}

func writeAttributes() {
	for name, cluster := range nodes {
		writeAttrs(name, cluster)
	}
}

func writeAttrs(name string, cluster map[string]*node) {
	attributeWrite, err := os.Create(common.GetFileNameByType(name, common.NodeEntityKind, common.Attributes))
	if err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.NodeEntityKind)
		return
	}

	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeEntityKind)
		}
	}(attributeWrite)

	if _, err = fmt.Fprintln(attributeWrite, "ClusterName,NodeName,VirtualTechnology,VirtualDomain,VirtualDatacenter,VirtualCluster,OsArchitecture,NetworkSpeed,CpuLimit,CpuRequest,MemoryLimit,MemoryRequest,CapacityPods,CapacityCpu,CapacityMemory,CapacityEphemeralStorage,CapacityHugePages,AllocatablePods,AllocatableCpu,AllocatableMemory,AllocatableEphemeralStorage,AllocatableHugePages,ProviderId,K8sVersion,NodeLabels,NodeTaints"); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.NodeEntityKind)
		return
	}

	for nodeName, n := range cluster {
		arch, region, zone := getArchRegionZone(n.labelMap)
		if _, err = fmt.Fprintf(attributeWrite, "%s,%s,Nodes,%s,%s,%s,%s", name, nodeName, name, region, zone, arch); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeEntityKind)
			return
		}
		values := []int{n.netSpeedBytes, n.cpuLimit, n.cpuRequest, n.memLimit, n.memRequest,
			n.podsCapacity, n.cpuCapacity, n.memCapacity, n.ephemeralStorageCapacity, n.hugepages2MiCapacity,
			n.podsAllocatable, n.cpuAllocatable, n.memAllocatable, n.ephemeralStorageAllocatable, n.hugepages2MiAllocatable}
		for _, value := range values {
			if err = common.PrintCSVIntValue(attributeWrite, value, false); err != nil {
				common.LogError(err, common.DefaultLogFormat, name, common.NodeEntityKind)
				return
			}
		}
		if err = common.PrintCSVStringValue(attributeWrite, n.providerId, false); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeEntityKind)
			return
		}
		if err = common.PrintCSVStringValue(attributeWrite, n.k8sVersion, false); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeEntityKind)
			return
		}
		if err = common.PrintCSVStringValue(attributeWrite, common.Empty, false); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeEntityKind)
			return
		}
		if err = common.PrintCSVLabelMap(attributeWrite, n.labelMap, false); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeEntityKind)
			return
		}
		if err = common.PrintCSVStringValue(attributeWrite, n.taints.String(), true); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeEntityKind)
			return
		}
	}
}

var nodeWorkloadWriters = common.NewWorkloadWriters()
