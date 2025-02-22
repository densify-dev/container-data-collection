// Package container2 collects data related to containers and formats into csv files to send to Densify.
package container

import (
	"fmt"
	"github.com/densify-dev/container-data-collection/internal/common"
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
				if err = common.PrintCSVPositiveNumberValue(configWrite, c.memory, false); err != nil {
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
	if _, err = fmt.Fprintln(attributeWrite, "ClusterName,Namespace,EntityName,EntityType,ContainerName,VirtualTechnology,VirtualDomain,VirtualDatacenter,VirtualCluster,ContainerLabels,PodLabels,CpuLimit,CpuRequest,MemoryLimit,MemoryRequest,ContainerName2,CurrentNodes,PowerState,CreatedByKind,CreatedByName,CurrentSize,CreateTime,ContainerRestarts,NamespaceLabels,NamespaceCpuRequest,NamespaceCpuLimit,NamespaceMemoryRequest,NamespaceMemoryLimit,NamespacePodsLimit,HpaName,HpaLabels,HpaTargetMetricName,HpaTargetMetricType,HpaTargetMetricValue,QosClass"); err != nil {
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
					if err = common.PrintCSVNumberValue(attributeWrite, value, false); err != nil {
						common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
						return
					}
				}
				if _, err = fmt.Fprintf(attributeWrite, ",%s,%s,%s,%s,%s", cName, common.ReplaceSemiColonsPipes(obj.labelMap[common.Node]), c.powerState.String(), getOwnerKindValue(obj.kind), obj.name); err != nil {
					common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
					return
				}
				if err = common.PrintCSVNumberValue(attributeWrite, obj.currentSize, false); err != nil {
					common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
					return
				}
				if err = common.PrintCSVTimeValue(attributeWrite, &obj.createTime, false); err != nil {
					common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
					return
				}
				if err = common.PrintCSVNumberValue(attributeWrite, c.restarts, false); err != nil {
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
				for _, value := range values {
					if err = common.PrintCSVNumberValue(attributeWrite, value, false); err != nil {
						common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
						return
					}
				}
				if err = obj.hpa.writeAttributes(attributeWrite, name, common.ContainerEntityKind, false); err != nil {
					return
				}
				if err = common.PrintCSVStringValue(attributeWrite, obj.qosClass, true); err != nil {
					common.LogError(err, common.DefaultLogFormat, name, common.ContainerEntityKind)
					return
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
	if _, err = fmt.Fprintln(attributeWrite, "ClusterName,Namespace,EntityName,EntityType,ContainerName,HpaName,HpaLabels,HpaTargetMetricName"); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.Hpa)
		return
	}
	for nsName, ns := range cluster {
		for _, h := range ns {
			if _, err = fmt.Fprintf(attributeWrite, "%s,%s,,,,", name, nsName); err != nil {
				common.LogError(err, common.DefaultLogFormat, name, common.Hpa)
				return
			}
			if err = h.writeAttributes(attributeWrite, name, common.Hpa, true); err != nil {
				return
			}
		}
	}
}

func (h *hpa) writeAttributes(attributeWrite *os.File, cluster, entityKind string, last bool) error {
	var err error
	if h == nil {
		if err = common.PrintCSVStringValue(attributeWrite, ",,,,", last); err != nil {
			common.LogError(err, common.DefaultLogFormat, cluster, entityKind)
			return err
		}
	} else {
		if err = common.PrintCSVStringValue(attributeWrite, h.name, false); err != nil {
			common.LogError(err, common.DefaultLogFormat, cluster, entityKind)
			return err
		}
		if err = common.PrintCSVStringValue(attributeWrite, common.Empty, false); err != nil {
			common.LogError(err, common.DefaultLogFormat, cluster, entityKind)
			return err
		}
		if err = common.PrintCSVLabelMap(attributeWrite, h.labels, false); err != nil {
			common.LogError(err, common.DefaultLogFormat, cluster, entityKind)
			return err
		}
		if err = common.PrintCSVStringValue(attributeWrite, h.metricName, false); err != nil {
			common.LogError(err, common.DefaultLogFormat, cluster, entityKind)
			return err
		}
		if err = common.PrintCSVStringValue(attributeWrite, h.metricTargetType, false); err != nil {
			common.LogError(err, common.DefaultLogFormat, cluster, entityKind)
			return err
		}
		if err = common.PrintCSVNumberValue(attributeWrite, h.metricTargetValue, last); err != nil {
			common.LogError(err, common.DefaultLogFormat, cluster, entityKind)
			return err
		}
	}
	return nil
}

var containerWorkloadWriters = common.NewWorkloadWriters()

var written = make(map[string]map[*container]bool)

var ownerLabelValues = []string{common.PodOwner, common.DeploymentOwner, common.JobOwner, common.NodeOwner,
	common.ReplicaSetOwner, common.DaemonSetOwner, common.StatefulSetOwner,
	common.ReplicationControllerOwner, common.CronJobOwner, common.ConfigMapOwner,
	// Well-known CRDs
	// - Operator Framework / OpenShift
	common.CatalogSourceOwner,
	// - Argo
	common.RolloutOwner, common.AnalysisRunOwner,
}

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
