package node

import (
	"fmt"
	"github.com/densify-dev/container-data-collection/internal/common"
	"os"
	"strings"
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
		if _, err = fmt.Fprintf(configWrite, "%s,%s,%s,%s,%s", common.FormatCurrentTime(), name, overrideNodeName(name, nodeName), instanceType, opSys); err != nil {
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
			if err = common.PrintCSVNumberValue(configWrite, value, i == last); err != nil {
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

	if _, err = fmt.Fprintln(attributeWrite, "ClusterName,NodeName,VirtualTechnology,VirtualDomain,VirtualDatacenter,VirtualCluster,OsArchitecture,NetworkSpeed,CpuLimit,CpuRequest,MemoryLimit,MemoryRequest,CapacityPods,CapacityCpu,CapacityMemory,CapacityEphemeralStorage,CapacityHugePages,AllocatablePods,AllocatableCpu,AllocatableMemory,AllocatableEphemeralStorage,AllocatableHugePages,MemoryTotalBytes,ProviderId,K8sVersion,NodeLabels,NodeTaints"); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.NodeEntityKind)
		return
	}

	for nodeName, n := range cluster {
		arch, region, zone := getArchRegionZone(n.labelMap)
		if _, err = fmt.Fprintf(attributeWrite, "%s,%s,Nodes,%s,%s,%s,%s", name, overrideNodeName(name, nodeName), name, region, zone, arch); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.NodeEntityKind)
			return
		}
		values := []int{n.netSpeedBytes, n.cpuLimit, n.cpuRequest, n.memLimit, n.memRequest,
			n.podsCapacity, n.cpuCapacity, n.memCapacity, n.ephemeralStorageCapacity, n.hugepages2MiCapacity,
			n.podsAllocatable, n.cpuAllocatable, n.memAllocatable, n.ephemeralStorageAllocatable, n.hugepages2MiAllocatable,
			n.memTotal}
		for _, value := range values {
			if err = common.PrintCSVNumberValue(attributeWrite, value, false); err != nil {
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

const (
	eksProviderIdPrefix = "aws:///"
	okeProviderIdPrefix = "ocid"
)

func overrideNodeName(cluster, nodeName string) (name string) {
	name = nodeName
	if n := nodes[cluster][nodeName]; n != nil {
		// EKS node names have the form:
		// ip-<IP address>.<AWS region>>.compute.internal
		// This is problematic (seen in practice) as IP addresses can be recycled, therefore
		// if a node has been torn down, another new node may get the same IP address.
		// EKS provider id have the format:
		// aws:///<availability zone>/<EC2 instance id>
		// EC2 instance id is unique so we add it to the node name
		//
		// OKE node names have the form:
		// <IP address>
		// This is problematic as IP addresses can be recycled, therefore
		// if a node has been torn down, another new node may get the same IP address.
		// OKE provider id have the form:
		// ocid1.instance.oc1.<region>.<unique id>
		// unique id is unique so we add it to the node name
		provId := strings.ToLower(n.providerId)
		if strings.HasPrefix(provId, eksProviderIdPrefix) {
			s := strings.Split(provId, "/")
			name += "--" + s[len(s)-1]
		} else if strings.HasPrefix(provId, okeProviderIdPrefix) {
			s := strings.Split(provId, ".")
			name += "--" + s[len(s)-1]
		}
	}
	return
}

// overrideNodeNameFieldsFunc assumes that the LAST field is the node name
func overrideNodeNameFieldsFunc(cluster string, fields []string) ([]string, bool) {
	l := len(fields)
	ok := l >= 1
	if ok {
		fields[l-1] = overrideNodeName(cluster, fields[l-1])
	}
	return fields, ok
}

func OverrideNodeNames(cluster string, nodes string, sep string) string {
	ns := strings.Split(nodes, sep)
	// git rid of possible trailing empty string
	if l := len(ns); l > 0 {
		if ns[l-1] == common.Empty {
			ns = ns[:l-1]
		}
		common.SortSlice(ns)
	}
	for i, n := range ns {
		ns[i] = overrideNodeName(cluster, n)
	}
	return strings.Join(ns, sep)
}

var nodeWorkloadWriters = common.NewWorkloadWriters()
