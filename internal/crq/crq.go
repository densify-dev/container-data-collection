package crq

import (
	"fmt"
	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/prometheus/common/model"
	"os"
	"time"
)

type crq struct {
	//Labels & general information about each node
	labelMap map[string]string

	selectorType, selectorKey, selectorValue                                                                                              string
	resources, namespaces                                                                                                                 string
	cpuLimit, cpuRequest, memLimit, memRequest, usageCpuLimit, usageCpuRequest, usageMemLimit, usageMemRequest, podsLimit, usagePodsLimit int
	createTime                                                                                                                            time.Time
}

var crqs = make(map[string]map[string]*crq)

const (
	labelCrq   = "name"
	labelKey   = "key"
	labelValue = "value"
)

func createCRQ(cluster string, result model.Matrix) {
	if _, f := crqs[cluster]; !f {
		if l := result.Len(); l > 0 {
			crqs[cluster] = make(map[string]*crq, l)
		}
	}
	for _, ss := range result {
		unixTimeInt := int64(ss.Values[len(ss.Values)-1].Value)
		crqName := string(ss.Metric[labelCrq])
		crqs[cluster][crqName] = &crq{
			labelMap:        make(map[string]string),
			cpuLimit:        common.UnknownValue,
			cpuRequest:      common.UnknownValue,
			memLimit:        common.UnknownValue,
			memRequest:      common.UnknownValue,
			usageCpuLimit:   common.UnknownValue,
			usageCpuRequest: common.UnknownValue,
			usageMemLimit:   common.UnknownValue,
			usageMemRequest: common.UnknownValue,
			podsLimit:       common.UnknownValue,
			usagePodsLimit:  common.UnknownValue,
			createTime:      time.Unix(unixTimeInt, 0),
		}
	}
}

func extractCRQAttributes(cluster string, result model.Matrix) {
	for _, val := range result {
		crqNameLabel, ok := val.Metric[labelCrq]
		if !ok {
			continue
		}
		crqName := string(crqNameLabel)
		if _, ok = crqs[cluster][crqName]; !ok {
			continue
		}
		for labelName, labelVal := range val.Metric {
			lv := string(labelVal)
			switch labelName {
			case common.Type:
				crqs[cluster][crqName].selectorType = lv
			case labelKey:
				crqs[cluster][crqName].selectorKey = lv
			case labelValue:
				crqs[cluster][crqName].selectorValue = lv
			case common.Namespace:
				crqs[cluster][crqName].namespaces += lv + "|"
			default: //Do nothing
			}
		}
	}
}

func getExistingQuotas(cluster string, result model.Matrix) {
	for _, val := range result {
		var crqNameLabel model.LabelValue
		var ok bool
		if crqNameLabel, ok = val.Metric[labelCrq]; !ok {
			continue
		}
		crqName := string(crqNameLabel)
		if _, ok := crqs[cluster][crqName]; !ok {
			continue
		}
		var resourceLabel model.LabelValue
		if resourceLabel, ok = val.Metric[common.Resource]; !ok {
			continue
		}
		resource := string(resourceLabel)
		lsv := common.LastSampleValue(val)
		v := float64(lsv)
		if typeHard := val.Metric[common.Type]; typeHard == common.Hard {
			crqs[cluster][crqName].resources += resource + ": " + lsv.String() + "|"
			switch resource {
			case common.LimitsCpu:
				crqs[cluster][crqName].cpuLimit = common.IntMCores(v)
			case common.RequestsCpu, common.Cpu:
				crqs[cluster][crqName].cpuRequest = common.IntMCores(v)
			case common.LimitsMem:
				crqs[cluster][crqName].memLimit = common.IntMiB(v)
			case common.RequestsMem, common.Memory:
				crqs[cluster][crqName].memRequest = common.IntMiB(v)
			case common.Pods:
				crqs[cluster][crqName].podsLimit = int(lsv)
			default:
			}
		} else if typeUsed := val.Metric[common.Type]; typeUsed == common.Used {
			switch resource {
			case common.LimitsCpu:
				crqs[cluster][crqName].usageCpuLimit = common.IntMCores(v)
			case common.RequestsCpu, common.Cpu:
				crqs[cluster][crqName].usageCpuRequest = common.IntMCores(v)
			case common.LimitsMem:
				crqs[cluster][crqName].usageMemLimit = common.IntMiB(v)
			case common.RequestsMem, common.Memory:
				crqs[cluster][crqName].usageMemRequest = common.IntMiB(v)
			case common.Pods:
				crqs[cluster][crqName].usagePodsLimit = int(lsv)
			default:
			}
		}
	}
}

func populateNameLabels(cluster string, result model.Matrix) {
	populateLabelMap(cluster, result, labelCrq)
}

// populateLabelMap is used to parse the label based results from Prometheus related to CRQ Entities and store them in the system's data structure.
func populateLabelMap(cluster string, result model.Matrix, nameLabel model.LabelName) {
	//Loop through the different entities in the results.
	for _, ss := range result {
		crqName, ok := ss.Metric[nameLabel]
		if !ok {
			continue
		}
		if _, ok := crqs[cluster][string(crqName)]; !ok {
			continue
		}
		for key, value := range ss.Metric {
			common.AddToLabelMap(string(key), string(value), crqs[cluster][string(crqName)].labelMap)
		}
	}
}

func writeConfig() {
	for name, cluster := range crqs {
		writeConf(name, cluster)
	}
}

func writeConf(name string, cluster map[string]*crq) {
	configWrite, err := os.Create(common.GetFileNameByType(name, common.CrqEntityKind, common.Config))
	if err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.CrqEntityKind)
		return
	}
	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.CrqEntityKind)
		}
	}(configWrite)
	if _, err = fmt.Fprintln(configWrite, "AuditTime,ClusterName,CrqName"); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.CrqEntityKind)
		return
	}
	for crqName := range cluster {
		if _, err = fmt.Fprintf(configWrite, "%s,%s,%s\n", common.FormatCurrentTime(), name, crqName); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.CrqEntityKind)
			return
		}
	}
}

func writeAttributes() {
	for name, cluster := range crqs {
		writeAttrs(name, cluster)
	}
}

func writeAttrs(name string, cluster map[string]*crq) {
	attributeWrite, err := os.Create(common.GetFileNameByType(name, common.CrqEntityKind, common.Attributes))
	if err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.CrqEntityKind)
		return
	}
	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.CrqEntityKind)
		}
	}(attributeWrite)
	if _, err = fmt.Fprintln(attributeWrite, "ClusterName,CrqName,VirtualTechnology,VirtualDomain,VirtualDatacenter,VirtualCluster,SelectorType,SelectorKey,SelectorValue,CreateTime,NamespaceLabels,ResourceMetadata,CpuLimit,CpuRequest,MemoryLimit,MemoryRequest,CurrentSize,NamespaceCpuLimit,NamespaceCpuRequest,NamespaceMemoryLimit,NamespaceMemoryRequest,NamespacePodsLimit,Namespaces"); err != nil {
		common.LogError(err, common.DefaultLogFormat, name, common.CrqEntityKind)
		return
	}
	for crqName, clrq := range cluster {
		if _, err = fmt.Fprintf(attributeWrite, "%s,%s,ClusterResourceQuota,%s,%s,%s,%s,%s,%s", name, crqName, name, clrq.selectorType, clrq.selectorKey, clrq.selectorType, clrq.selectorKey, clrq.selectorValue); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.CrqEntityKind)
			return
		}
		if err = common.PrintCSVTimeValue(attributeWrite, &clrq.createTime, false); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.CrqEntityKind)
			return
		}
		if _, err = fmt.Fprint(attributeWrite, ","); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.CrqEntityKind)
			return
		}
		if err = common.PrintCSVLabelMap(attributeWrite, clrq.labelMap, false); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.CrqEntityKind)
			return
		}
		if _, err = fmt.Fprintf(attributeWrite, ",%s", clrq.resources); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.CrqEntityKind)
			return
		}
		values := []int{clrq.usageCpuLimit, clrq.usageCpuRequest, clrq.usageMemLimit, clrq.usageMemRequest, clrq.usagePodsLimit,
			clrq.cpuLimit, clrq.cpuRequest, clrq.memLimit, clrq.memRequest, clrq.podsLimit}
		for _, value := range values {
			if err = common.PrintCSVIntValue(attributeWrite, value, false); err != nil {
				common.LogError(err, common.DefaultLogFormat, name, common.CrqEntityKind)
				return
			}
		}
		if _, err = fmt.Fprintf(attributeWrite, ",%s\n", clrq.namespaces); err != nil {
			common.LogError(err, common.DefaultLogFormat, name, common.CrqEntityKind)
			return
		}
	}
}

// Metrics a global func for collecting quota level metrics in prometheus
func Metrics() {
	var query string

	//Start and end time + the prometheus address used for querying
	range5Min := common.TimeRange()

	query = `max(openshift_clusterresourcequota_created{}) by (namespace,name)`
	if n, err := common.CollectAndProcessMetric(query, range5Min, createCRQ); err != nil || n == 0 {
		// error already handled
		return
	}

	// for all other queries we ignore failures
	query = `max(openshift_clusterresourcequota_selector{}) by (name, key, type, value)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, extractCRQAttributes)

	query = `openshift_clusterresourcequota_labels{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, populateNameLabels)

	query = `max(openshift_clusterresourcequota_usage{}) by (name, resource, type)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, getExistingQuotas)

	query = `max(openshift_clusterresourcequota_namespace_usage{}) by (name, namespace)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, extractCRQAttributes)

	writeConfig()
	writeAttributes()

	var metricField = []model.LabelName{labelCrq}
	query = `sum(openshift_clusterresourcequota_usage{type="used", resource="limits.cpu"}) by (name) * 1000`
	common.CpuLimits.GetWorkload(query, metricField, common.CrqEntityKind)

	query = `sum(openshift_clusterresourcequota_usage{type="used", resource=~"cpu|requests\\.cpu"}) by (name) * 1000`
	common.CpuRequests.GetWorkload(query, metricField, common.CrqEntityKind)

	query = `sum(openshift_clusterresourcequota_usage{type="used", resource="limits.memory"}) by (name)`
	common.MemLimits.GetWorkload(query, metricField, common.CrqEntityKind)

	query = `sum(openshift_clusterresourcequota_usage{type="used", resource=~"memory|requests\\.memory"}) by (name) / (1024 * 1024)`
	common.MemRequests.GetWorkload(query, metricField, common.CrqEntityKind)

	query = `sum(openshift_clusterresourcequota_usage{type="used", resource="pods"}) by (name)`
	common.PodsLimits.GetWorkload(query, metricField, common.CrqEntityKind)
}
