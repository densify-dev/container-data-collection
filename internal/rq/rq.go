package rq

import (
	"fmt"
	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/prometheus/common/model"
	"os"
	"strconv"
	"time"
)

type resourceQuota struct {
	resources string
	cpuLimit, cpuRequest, memLimit, memRequest, podsLimit,
	usageCpuLimit, usageCpuRequest, usageMemLimit, usageMemRequest, usagePodsLimit int
	createTime time.Time
}
type namespace struct {
	rqs map[string]*resourceQuota
}

var resourceQuotas = make(map[string]map[string]*namespace)

const (
	labelRQ = "resourcequota"
)

func getResourceQuota(cluster string, ss *model.SampleStream) (rq *resourceQuota) {
	var nsNameLabel, rqNameLabel model.LabelValue
	var ok bool
	if nsNameLabel, ok = ss.Metric[common.Namespace]; !ok {
		return
	}
	nsName := string(nsNameLabel)
	if rqNameLabel, ok = ss.Metric[labelRQ]; !ok {
		return
	}
	rqName := string(rqNameLabel)
	var cl map[string]*namespace
	if cl, ok = resourceQuotas[cluster]; !ok {
		cl = make(map[string]*namespace)
		resourceQuotas[cluster] = cl
	}
	var ns *namespace
	if ns, ok = cl[nsName]; !ok {
		ns = &namespace{rqs: make(map[string]*resourceQuota)}
		cl[nsName] = ns
	}
	if rq, ok = ns.rqs[rqName]; !ok {
		rq = &resourceQuota{
			cpuLimit:        common.UnknownValue,
			cpuRequest:      common.UnknownValue,
			memLimit:        common.UnknownValue,
			memRequest:      common.UnknownValue,
			podsLimit:       common.UnknownValue,
			usageCpuLimit:   common.UnknownValue,
			usageCpuRequest: common.UnknownValue,
			usageMemLimit:   common.UnknownValue,
			usageMemRequest: common.UnknownValue,
			usagePodsLimit:  common.UnknownValue,
		}
		ns.rqs[rqName] = rq
	}
	return
}

func getExistingQuotas(cluster string, result model.Matrix) {
	for _, val := range result {
		var rq *resourceQuota
		if rq = getResourceQuota(cluster, val); rq == nil {
			continue
		}
		var resource string
		var ok bool
		if resource, ok = common.GetLabelValue(val, common.Resource); !ok {
			continue
		}
		value := common.LastValue(val)
		if typeHard := val.Metric[common.Type]; typeHard == common.Hard {
			rq.resources += resource + ": " + strconv.FormatFloat(value, 'f', 2, 64) + "|"
			switch resource {
			case common.LimitsCpu:
				rq.cpuLimit = common.IntMCores(value)
			case common.RequestsCpu, common.Cpu:
				rq.cpuRequest = common.IntMCores(value)
			case common.LimitsMem:
				rq.memLimit = common.IntMiB(value)
			case common.RequestsMem, common.Memory:
				rq.memRequest = common.IntMiB(value)
			case common.CountPods, common.Pods:
				rq.podsLimit = int(value)
			default:
			}
		} else if typeUsed := val.Metric[common.Type]; typeUsed == common.Used {
			switch resource {
			case common.LimitsCpu:
				rq.usageCpuLimit = common.IntMCores(value)
			case common.RequestsCpu, common.Cpu:
				rq.usageCpuRequest = common.IntMCores(value)
			case common.LimitsMem:
				rq.usageMemLimit = common.IntMiB(value)
			case common.RequestsMem, common.Memory:
				rq.usageMemRequest = common.IntMiB(value)
			case common.CountPods, common.Pods:
				rq.usagePodsLimit = int(value)
			default:
			}
		}
	}
}

func getCreationTime(cluster string, result model.Matrix) {
	for _, val := range result {
		if rq := getResourceQuota(cluster, val); rq == nil {
			continue
		} else if sec := common.LastValue(val); sec != 0 {
			rq.createTime = time.Unix(int64(sec), 0)
		}
	}
}

func writeConfig() {
	for name, cluster := range resourceQuotas {
		writeConf(name, cluster)
	}
}

func writeConf(name string, cluster map[string]*namespace) {
	configWrite, err := os.Create(common.GetFileNameByType(name, common.RqEntityKind, common.Config))
	if err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.RqEntityKind)
		return
	}
	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.RqEntityKind)
		}
	}(configWrite)
	if _, err = fmt.Fprintln(configWrite, "AuditTime,ClusterName,Namespace,RqName"); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.RqEntityKind)
		return
	}
	for nsName, ns := range cluster {
		for rqName := range ns.rqs {
			if _, err = fmt.Fprintf(configWrite, "%s,%s,%s,%s\n", common.FormatCurrentTime(), name, nsName, rqName); err != nil {
				common.LogError(err, common.DefaultLogFormat, name, common.RqEntityKind)
				return
			}
		}
	}
}

func writeAttributes() {
	for name, cluster := range resourceQuotas {
		writeAttrs(name, cluster)
	}
}

func writeAttrs(name string, cluster map[string]*namespace) {
	attributeWrite, err := os.Create(common.GetFileNameByType(name, common.RqEntityKind, common.Attributes))
	if err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.RqEntityKind)
		return
	}
	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.RqEntityKind)
		}
	}(attributeWrite)
	if _, err = fmt.Fprintln(attributeWrite, "ClusterName,Namespace,RqName,VirtualTechnology,VirtualDomain,VirtualDatacenter,CreateTime,ResourceMetadata,CpuLimit,CpuRequest,MemoryLimit,MemoryRequest,CurrentSize,NamespaceCpuLimit,NamespaceCpuRequest,NamespaceMemoryLimit,NamespaceMemoryRequest,NamespacePodsLimit"); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.RqEntityKind)
		return
	}
	for nsName, ns := range cluster {
		for rqName, rq := range ns.rqs {
			if _, err = fmt.Fprintf(attributeWrite, "%s,%s,%s,ResourceQuota,%s,%s", name, nsName, rqName, name, nsName); err != nil {
				common.LogError(err, common.DefaultLogFormat, name, common.RqEntityKind)
				return
			}
			if err = common.PrintCSVTimeValue(attributeWrite, &rq.createTime, false); err != nil {
				common.LogError(err, common.DefaultLogFormat, name, common.RqEntityKind)
				return
			}
			if _, err = fmt.Fprintf(attributeWrite, ",%s", rq.resources); err != nil {
				common.LogError(err, common.DefaultLogFormat, name, common.RqEntityKind)
				return
			}
			values := []int{rq.usageCpuLimit, rq.usageCpuRequest, rq.usageMemLimit, rq.usageMemRequest, rq.usagePodsLimit,
				rq.cpuLimit, rq.cpuRequest, rq.memLimit, rq.memRequest, rq.podsLimit}
			last := len(values) - 1
			for i, value := range values {
				if err = common.PrintCSVNumberValue(attributeWrite, value, i == last); err != nil {
					common.LogError(err, common.DefaultLogFormat, name, common.RqEntityKind)
					return
				}
			}
		}
	}
}

// Metrics a global func for collecting quota level metrics in prometheus
func Metrics() {
	var query string
	range5Min := common.TimeRange()

	query = `max(kube_resourcequota{}) by (resourcequota, resource, namespace, type)`
	if n, err := common.CollectAndProcessMetric(query, range5Min, getExistingQuotas); err != nil || n == 0 {
		// error already handled
		return
	}

	// for all other queries we ignore failures
	query = `max(kube_resourcequota_created{}) by (namespace,resourcequota)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, getCreationTime)

	writeAttributes()
	writeConfig()

	var metricField = []model.LabelName{common.Namespace, labelRQ}

	query = `sum(kube_resourcequota{type="used", resource="limits.cpu"}) by (resourcequota,namespace) * 1000`
	common.CpuLimits.GetWorkload(query, metricField, common.RqEntityKind)

	query = `sum(kube_resourcequota{type="used", resource=~"cpu|requests\\.cpu"}) by (resourcequota,namespace) * 1000`
	common.CpuRequests.GetWorkload(query, metricField, common.RqEntityKind)

	query = `sum(kube_resourcequota{type="used", resource="limits.memory"}) by (resourcequota,namespace)`
	common.MemLimits.GetWorkload(query, metricField, common.RqEntityKind)

	query = `sum(kube_resourcequota{type="used", resource=~"memory|requests\\.memory"}) by (resourcequota,namespace) / (1024 * 1024)`
	common.MemRequests.GetWorkload(query, metricField, common.RqEntityKind)

	query = `sum(kube_resourcequota{type="used", resource=~"pods|count\\/pods"}) by (resourcequota,namespace)`
	common.PodsLimits.GetWorkload(query, metricField, common.RqEntityKind)
}
