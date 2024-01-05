// Package container2 collects data related to containers and formats into csv files to send to Densify.
package container

import (
	"fmt"
	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/prometheus/common/model"
	"os"
	"strings"
)

type namespacesWrite func(name string, cluster map[string]*namespace)

type hpaWrite func(name string, cluster map[string]map[string]*hpa)

func writeConfig() {
	write(writeConf, writeHpaConf)
}

func writeAttributes() {
	write(writeAttrs, writeHpaAttrs)
}

func write(nw namespacesWrite, hw hpaWrite) {
	if nw != nil {
		for name, cluster := range namespaces {
			nw(name, cluster)
		}
	}
	if hw != nil {
		for name, cluster := range unclassifiedHpas {
			hw(name, cluster)
		}
	}
}

func writeConf(name string, cluster map[string]*namespace) {
	configWrite, err := os.Create(common.GetFileNameByType(name, common.ContainerEntityKind, common.Config))
	if err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
		return
	}
	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
		}
	}(configWrite)
	if _, err = fmt.Fprintln(configWrite, "AuditTime,ClusterName,Namespace,EntityName,EntityType,ContainerName,HwTotalMemory,OsName,HwManufacturer"); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
		return
	}
	for nsName, ns := range cluster {
		for _, obj := range ns.objects {
			for cName, c := range obj.containers {
				if _, err = fmt.Fprintf(configWrite, "%s,%s,%s,%s,%s,%s", common.FormatCurrentTime(), name, nsName, obj.name, getOwnerKindValue(obj.kind), common.ReplaceColons(cName)); err != nil {
					common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
					return
				}
				if err = common.PrintCSVPositiveIntValue(configWrite, c.memory, false); err != nil {
					common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
					return
				}
				if _, err = fmt.Fprintln(configWrite, ",Linux,CONTAINERS"); err != nil {
					common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
					return
				}
			}
		}
	}
}

func writeHpaConf(name string, cluster map[string]map[string]*hpa) {
	if len(cluster) == 0 {
		common.LogCluster(1, common.Info, "no HPA found for cluster %s", name, true, name)
		return
	}
	configWrite, err := os.Create(common.GetExtraFileNameByType(name, common.Hpa, common.Config))
	if err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.Hpa)
		return
	}
	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.Hpa)
		}
	}(configWrite)
	if _, err = fmt.Fprintln(configWrite, "AuditTime,ClusterName,Namespace,EntityName,EntityType,ContainerName,HpaName,OsName,HwManufacturer"); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.Hpa)
		return
	}
	for nsName, ns := range cluster {
		for hpaName := range ns {
			if _, err = fmt.Fprintf(configWrite, "%s,%s,%s,,,,%s,Linux,HPA\n", common.FormatCurrentTime(), name, nsName, hpaName); err != nil {
				common.LogError(err, common.DefaultLogFormat, name, common.Hpa)
				return
			}
		}
	}
}

func writeAttrs(name string, cluster map[string]*namespace) {
	attributeWrite, err := os.Create(common.GetFileNameByType(name, common.ContainerEntityKind, common.Attributes))
	if err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
		return
	}
	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
		}
	}(attributeWrite)
	if _, err = fmt.Fprintln(attributeWrite, "ClusterName,Namespace,EntityName,EntityType,ContainerName,VirtualTechnology,VirtualDomain,VirtualDatacenter,VirtualCluster,ContainerLabels,PodLabels,CpuLimit,CpuRequest,MemoryLimit,MemoryRequest,ContainerName2,CurrentNodes,PowerState,CreatedByKind,CreatedByName,CurrentSize,CreateTime,ContainerRestarts,NamespaceLabels,NamespaceCpuRequest,NamespaceCpuLimit,NamespaceMemoryRequest,NamespaceMemoryLimit,NamespacePodsLimit"); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
		return
	}
	for nsName, ns := range cluster {
		for _, obj := range ns.objects {
			for cName, c := range obj.containers {
				if _, err = fmt.Fprintf(attributeWrite, "%s,%s,%s,%s,%s,Containers,%s,%s,%s,", name, nsName, common.ReplaceSemiColons(obj.name), getOwnerKindValue(obj.kind), common.ReplaceColons(cName), name, nsName, obj.name); err != nil {
					common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
					return
				}
				if err = common.ConditionalPrintCSVLabelMap(attributeWrite, c.labelMap, false, rejectKeys); err != nil {
					common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
					return
				}
				if err = common.PrintCSVStringValue(attributeWrite, common.Empty, false); err != nil {
					common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
					return
				}
				if err = common.ConditionalPrintCSVLabelMap(attributeWrite, obj.labelMap, false, rejectKeys); err != nil {
					common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
					return
				}
				values := []int{c.cpuLimit, c.cpuRequest, c.memLimit, c.memRequest}
				for _, value := range values {
					if err = common.PrintCSVIntValue(attributeWrite, value, false); err != nil {
						common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
						return
					}
				}
				if _, err = fmt.Fprintf(attributeWrite, ",%s,%s,%v,%s,%s", cName, common.ReplaceSemiColonsPipes(obj.labelMap[common.Node]), c.powerState, getOwnerKindValue(obj.kind), obj.name); err != nil {
					common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
					return
				}
				if err = common.PrintCSVIntValue(attributeWrite, obj.currentSize, false); err != nil {
					common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
					return
				}
				if err = common.PrintCSVTimeValue(attributeWrite, &obj.createTime, false); err != nil {
					common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
					return
				}
				if err = common.PrintCSVIntValue(attributeWrite, c.restarts, false); err != nil {
					common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
					return
				}
				if err = common.PrintCSVStringValue(attributeWrite, common.Empty, false); err != nil {
					common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
					return
				}
				if err = common.ConditionalPrintCSVLabelMap(attributeWrite, ns.labelMap, false, rejectKeys); err != nil {
					common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
					return
				}
				values = []int{ns.cpuRequest, ns.cpuLimit, ns.memRequest, ns.memLimit, ns.podsLimit}
				lastOne := len(values) - 1
				for i, value := range values {
					if err = common.PrintCSVIntValue(attributeWrite, value, i == lastOne); err != nil {
						common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
						return
					}
				}
			}
		}
	}
}

const (
	kubeLastAppliedConfLabel = "annotation_kubectl_kubernetes_io_last_applied_configuration"
)

var rejectKeys = map[string]bool{kubeLastAppliedConfLabel: true}

func writeHpaAttrs(name string, cluster map[string]map[string]*hpa) {
	if len(cluster) == 0 {
		return
	}
	attributeWrite, err := os.Create(common.GetExtraFileNameByType(name, common.Hpa, common.Attributes))
	if err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.Hpa)
		return
	}
	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.Hpa)
		}
	}(attributeWrite)
	if _, err = fmt.Fprintln(attributeWrite, "ClusterName,Namespace,EntityName,EntityType,ContainerName,HpaName,Labels"); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.Hpa)
		return
	}
	for nsName, ns := range cluster {
		for hpaName, h := range ns {
			if _, err = fmt.Fprintf(attributeWrite, "%s,%s,,,,%s,", name, nsName, hpaName); err != nil {
				common.LogError(err, common.DefaultLogFormat, name, common.Hpa)
				return
			}
			if err = common.PrintCSVLabelMap(attributeWrite, h.labels, true); err != nil {
				common.LogError(err, common.DefaultLogFormat, name, common.Hpa)
				return
			}
		}
	}
}

var objWorkloadWriters = make(map[string]map[string]*os.File)

func writeObjWorkload(metric, cluster, nsName string, obj *k8sObject, values []model.SamplePair) (err error) {
	if file := objWorkloadWriters[metric][cluster]; file != nil {
	outer:
		for cName := range obj.containers {
			for _, value := range values {
				if _, err = fmt.Fprintf(file, "%s,%s,%s,%s,%s,%s,%f\n", cluster, nsName, obj.name, getOwnerKindValue(obj.kind), common.ReplaceColons(cName), common.FormatTime(value.Timestamp), value.Value); err != nil {
					common.LogError(err, common.DefaultLogFormat, cluster, common.ContainerEntityKind)
					break outer
				}
			}
		}
	}
	return
}

var ownerLabelValues = []string{common.PodOwner, common.DeploymentOwner, common.JobOwner, common.NodeOwner,
	common.ReplicaSetOwner, common.DaemonSetOwner, common.StatefulSetOwner,
	common.ReplicationControllerOwner, common.CronJobOwner, common.ConfigMapOwner, common.CatalogSourceOwner}

var ownerLabelValuesMap = makeOwnerLabelValuesMap()

func makeOwnerLabelValuesMap() map[string]string {
	m := make(map[string]string, len(ownerLabelValues))
	for _, ownerLabelValue := range ownerLabelValues {
		m[strings.ToLower(ownerLabelValue)] = ownerLabelValue
	}
	return m
}

func getOwnerKindValue(kind string) (k string) {
	if olv, f := ownerLabelValuesMap[kind]; f {
		k = olv
	} else {
		k = kind
	}
	return
}

var hpaWorkloadEntityTypes = map[bool]string{true: common.ContainerEntityKind, false: common.Hpa}
