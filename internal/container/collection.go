package container

import (
	"fmt"
	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/densify-dev/container-data-collection/internal/node"
	"github.com/prometheus/common/model"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	nsIdx = iota
	podIdx
	containerIdx
)

const (
	labelReplacePrefix    = `label_replace(`
	labelReplaceSuffixFmt = `, "%s", "$1", "%s", "(.*)")`
)

type labelHolder struct {
	detected bool
	names    []string
	wqws     map[int]*common.WorkloadQueryWrapper
}

func labelPlaceholder(s string) string {
	return common.CamelCase(s, common.Label)
}

var labelPlaceholders = []string{common.Empty, labelPlaceholder(common.Pod), labelPlaceholder(common.Container)}

func (lh *labelHolder) values(ss *model.SampleStream) (string, string, string, bool) {
	vals, ok := common.GetLabelsValues(ss, lh.names)
	return vals[common.Namespace], vals[common.Pod], vals[common.Container], ok
}

func (lh *labelHolder) setWrapper(lh1, lh2 *labelHolder, l int) {
	lh.wqws[l] = &common.WorkloadQueryWrapper{
		Prefix: labelReplacePrefix,
		Suffix: fmt.Sprintf(labelReplaceSuffixFmt, lh1.names[l], lh2.names[l]),
	}
}

type labelNamesType int

type queryProcessorBuilder struct {
	lnt labelNamesType
	th  *typeHolder
}

const (
	podLabelNames labelNamesType = iota
	fullOwnerLabelNames
	typeOwnerLabelNames
)

func (lh *labelHolder) getQueryProcessor(qpb *queryProcessorBuilder) (qp *common.QueryProcessor) {
	if lh == nil || qpb == nil {
		return
	}
	var nms []model.LabelName
	switch qpb.lnt {
	case podLabelNames:
		nms = stdPodLabels
	case fullOwnerLabelNames:
		nms = fullOwnerLabels
	case typeOwnerLabelNames:
		nms = typeOwnerLabels
	default:
		return
	}
	names := append(nms, model.LabelName(lh.names[containerIdx]))
	var ff common.FieldsFunc
	if qpb.th != nil {
		ff = qpb.th.containerFields
	}
	qp = &common.QueryProcessor{
		MetricFields: names,
		FF:           ff,
	}
	return
}

var stdLabelHolder = &labelHolder{names: []string{common.Namespace, common.Pod, common.Container}}
var stdPodLabels = []model.LabelName{common.Namespace, common.Pod}
var typeOwnerLabels = []model.LabelName{common.Namespace, model.LabelName(ownerName)}
var fullOwnerLabels = append(typeOwnerLabels, model.LabelName(ownerKind))

var nameLabelHolder = &labelHolder{
	names: []string{common.Namespace, common.SnakeCase(common.Pod, common.Name), common.SnakeCase(common.Container, common.Name)},
	wqws:  make(map[int]*common.WorkloadQueryWrapper),
}
var labelHolders = makeLabelHolders()

func makeLabelHolders() []*labelHolder {
	// set wrappers for nameLabelHolder
	nameLabelHolder.setWrapper(stdLabelHolder, nameLabelHolder, podIdx)
	nameLabelHolder.setWrapper(nameLabelHolder, stdLabelHolder, containerIdx)
	return []*labelHolder{stdLabelHolder, nameLabelHolder}
}

var clusterLabelHolders = make(map[string]*labelHolder)

type metricHolder struct {
	metric string
}

var metricRequireSameObject = map[string]bool{createTime: true, common.CurrentSizeName: true}

func getOwnerId(cluster, nsName string, objId *objectId) (ownerId *objectId, ok bool) {
	if objId != nil {
		var o *ownership
		if o, ok = ownerships[cluster][objId.Key(nsName)]; ok {
			ownerId = o.getTopLevelOwner()
		} else {
			ownerId, ok = objId, true
		}
	}
	return
}

func getOwner(cluster, nsName string, ns *namespace, objId *objectId) (obj *k8sObject, ok bool) {
	if ns != nil {
		var ownerId *objectId
		if ownerId, ok = getOwnerId(cluster, nsName, objId); ok {
			obj, ok = ns.objects[ownerId.Key(nsName)]
		}
	}
	return
}

func getContainer(cluster string, ss *model.SampleStream) (c *container, cwp *containerWorkloadProducer, ok bool) {
	var nsName, podName, containerName string
	if clh, f := clusterLabelHolders[cluster]; f {
		nsName, podName, containerName, ok = clh.values(ss)
	} else {
		for _, lh := range labelHolders {
			if nsName, podName, containerName, ok = lh.values(ss); ok {
				lh.detected = true
				clusterLabelHolders[cluster] = lh
				break
			}
		}
	}
	if ok {
		var ns *namespace
		if ns, ok = namespaces[cluster][nsName]; ok {
			podId := &objectId{kind: common.Pod, name: podName}
			var owner *k8sObject
			if owner, ok = getOwner(cluster, nsName, ns, podId); ok {
				if c, ok = owner.containers[containerName]; ok {
					cwp = &containerWorkloadProducer{cluster: cluster, nsName: nsName, obj: owner, c: c}
				}
			}
		}
	}
	return
}

func (mh *metricHolder) getContainerMetric(cluster string, result model.Matrix) {
	for _, ss := range result {
		c, cwp, ok := getContainer(cluster, ss)
		if !ok {
			continue
		}
		value := common.LastValue(ss)
		resource, _ := common.GetLabelValue(ss, common.Resource)
		switch mh.metric {
		case common.Limits:
			switch resource {
			case common.Memory:
				c.memLimit = common.IntMiB(value)
				common.WriteWorkload(cwp, containerWorkloadWriters, common.MemoryLimits, ss, common.MiB[float64])
			case common.Cpu:
				c.cpuLimit = common.IntMCores(value)
				common.WriteWorkload(cwp, containerWorkloadWriters, common.CpuLimits, ss, common.MCores[float64])
			case common.NvidiaGpuResource:
				c.gpuLimit = int(value)
				common.WriteWorkload(cwp, containerWorkloadWriters, common.GpuLimits, ss, nil)
			}
		case common.Requests:
			switch resource {
			case common.Memory:
				c.memRequest = common.IntMiB(value)
				common.WriteWorkload(cwp, containerWorkloadWriters, common.MemoryRequests, ss, common.MiB[float64])
			case common.Cpu:
				c.cpuRequest = common.IntMCores(value)
				common.WriteWorkload(cwp, containerWorkloadWriters, common.CpuRequests, ss, common.MCores[float64])
			case common.NvidiaGpuResource:
				c.gpuRequest = int(value)
				common.WriteWorkload(cwp, containerWorkloadWriters, common.GpuRequests, ss, nil)
			}
		case common.Memory:
			c.memory = common.IntMiB(value)
			// skip an extra call to getContainerMetricString
			addToLabelMap(ss.Metric, c.labelMap, excludeNodeLabel)
		case common.GpuMemoryTotal:
			// add the value to the total
			if c.gpuMemTotal == common.UnknownValue {
				c.gpuMemTotal = 0
			}
			c.gpuMemTotal += int(value)
			// also get the GPU model name && sharing strategy
			concatenateValue(ss, common.ModelName, &c.gpuModel, nil, nil)
			np := &nodeProvider{cluster: cluster}
			concatenateValue(ss, common.Node, &c.gpuSharingStrategy, np.getGpuSharingStrategy, nil)
		case common.CpuLimit:
			c.cpuLimit = common.IntMCores(value)
			common.WriteWorkload(cwp, containerWorkloadWriters, common.CpuLimits, ss, common.MCores[float64])
		case common.CpuRequest:
			c.cpuRequest = common.IntMCores(value)
			common.WriteWorkload(cwp, containerWorkloadWriters, common.CpuRequests, ss, common.MCores[float64])
		case common.MemLimit:
			c.memLimit = common.IntMiB(value)
			common.WriteWorkload(cwp, containerWorkloadWriters, common.MemoryLimits, ss, common.MiB[float64])
		case common.MemRequest:
			c.memRequest = common.IntMiB(value)
			common.WriteWorkload(cwp, containerWorkloadWriters, common.CpuRequests, ss, common.MCores[float64])
		case restarts:
			c.restarts += int(value)
		case powerSt:
			c.powerState = powerState(value)
		}
	}
}

type valueFunc func(string) string
type equalFunc includeFunc

type nodeProvider struct {
	cluster string
}

func (np *nodeProvider) getGpuSharingStrategy(name string) string {
	return node.GetNodeGpuSharingStrategy(np.cluster, name)
}

func concatenateValue(ss *model.SampleStream, labelName string, value *string, vf valueFunc, ef equalFunc) {
	if v, f := common.GetLabelValue(ss, labelName); f {
		if vf != nil {
			v = vf(v)
		}
		if v == common.Empty {
			return
		}
		if ef == nil {
			ef = strings.EqualFold
		}
		ms := strings.Split(*value, common.Or)
		f = false
		for _, m := range ms {
			if f = ef(m, v); f {
				break
			}
		}
		if !f {
			if len(ms) == 1 && ms[0] == common.Empty {
				ms = nil
			}
			ms = append(ms, v)
			sort.Strings(ms)
			*value = strings.Join(ms, common.Or)
		}
	}
}

func getContainerMetricString(cluster string, result model.Matrix) {
	for _, ss := range result {
		if c, _, ok := getContainer(cluster, ss); ok {
			addToLabelMap(ss.Metric, c.labelMap, excludeNodeLabel)
		}
	}
}

func getNamespaceAndValue(th *typeHolder, cluster string, ss *model.SampleStream) (nsName string, ns *namespace, value string, ok bool) {
	var ln string
	if th != nil {
		ln = th.getTypeLabelName()
	}
	hasLabel := ln != common.Empty
	var lns []string
	if hasLabel {
		lns = []string{ln}
	}
	var values map[string]string
	nsName, ns, values, ok = getNamespaceAndValues(lns, cluster, ss)
	if ok && hasLabel {
		value = values[ln]
	}
	return
}

func getNamespaceAndValues(lns []string, cluster string, ss *model.SampleStream) (nsName string, ns *namespace, values map[string]string, ok bool) {
	names := append([]string{common.Namespace}, lns...)
	if values, ok = common.GetLabelsValues(ss, names); !ok {
		return
	}
	nsName = values[common.Namespace]
	if cluster != common.Empty {
		if ns, ok = namespaces[cluster][nsName]; !ok {
			return
		}
	}
	return
}

func isRelevant(cluster, nsName string, oid *objectId) (b bool) {
	if oid != nil {
		b = oid.kind == common.Pod || detectedOwners[cluster][oid.Key(nsName)]
	}
	return
}

func (th *typeHolder) getObject(cluster string, ss *model.SampleStream, requiresSameObject bool) (nsName string, obj *k8sObject, ok bool) {
	var ns *namespace
	var value string
	if nsName, ns, value, ok = getNamespaceAndValue(th, cluster, ss); ok {
		obj, ok = getObject(cluster, nsName, th.typeName, value, ns, requiresSameObject)
	}
	return
}

func getObject(cluster, nsName, kind, name string, ns *namespace, requiresSameObject bool) (obj *k8sObject, ok bool) {
	oid := &objectId{kind: kind, name: name}
	if ok = isRelevant(cluster, nsName, oid); ok {
		if obj, ok = getOwner(cluster, nsName, ns, oid); ok && requiresSameObject {
			ok = obj.kind == oid.kind && obj.name == oid.name
		}
	}
	return
}

type objectMetricHolder struct {
	*metricHolder
	*typeHolder
}

func (omh *objectMetricHolder) getObjectMetric(cluster string, result model.Matrix) {
	for _, ss := range result {
		nsName, obj, ok := omh.getObject(cluster, ss, metricRequireSameObject[omh.metric])
		if !ok {
			continue
		}
		value := common.LastValue(ss)
		int64Value := int64(value)
		switch omh.metric {
		case common.CurrentSizeName:
			// need to set the maximum value
			obj.currentSize = int(int64Value)
			cwp := &containerWorkloadProducer{cluster: cluster, nsName: nsName, obj: obj}
			common.WriteWorkload(cwp, containerWorkloadWriters, common.CurrentSize, ss, nil)
		case createTime:
			obj.createTime = time.Unix(int64Value, 0)
		case qosClass:
			// as we set a single value as an attribute, set the maximum value we find
			qosClassValue := string(ss.Metric[model.LabelName(qosClassLabel)])
			if cmpQosClasses(qosClassValue, obj.qosClass) > 0 {
				obj.qosClass = qosClassValue
			}
		default:
			common.AddToLabelMap(omh.metric, strconv.FormatInt(int64Value, 10), obj.labelMap)
		}
	}
}

type ownedObjectMetricHolder struct {
	*objectMetricHolder
}

func (oomh *ownedObjectMetricHolder) getObjectMetric(cluster string, result model.Matrix) {
	for _, omh := range oomh.getOwnerObjectMetricHolders(cluster) {
		omh.getObjectMetric(cluster, result)
	}
}

func (oomh *ownedObjectMetricHolder) getOwnerObjectMetricHolders(cluster string) (omhs []*objectMetricHolder) {
	for ownerType := range detectedOwnershipTypes[oomh.typeName][cluster] {
		omhs = append(omhs, &objectMetricHolder{oomh.metricHolder, &typeHolder{typeName: ownerType, typeLabel: ownerName}})
	}
	return
}

type hpaMetricHolder struct {
	*objectMetricHolder
}

func (hmh *hpaMetricHolder) query(suffix ...string) string {
	return common.SnakeCase(append([]string{kube, hmh.getTypeLabelName()}, suffix...)...)
}

func (th *typeHolder) getObjectMetricStringIncludeAll(cluster string, result model.Matrix) {
	th.getObjectMetricStringInclude(cluster, result, nil)
}

func (th *typeHolder) getObjectMetricString(cluster string, result model.Matrix) {
	// https://github.com/kubernetes/kube-state-metrics/issues/1927
	// only in kube_pod_info node label is overridden with the true value
	th.getObjectMetricStringInclude(cluster, result, excludeNodeLabel)
}

func (th *typeHolder) getObjectMetricStringInclude(cluster string, result model.Matrix, f includeFunc) {
	for _, ss := range result {
		_, obj, ok := th.getObject(cluster, ss, false)
		if !ok {
			continue
		}
		addToLabelMap(ss.Metric, obj.labelMap, f)
	}
}

func addToLabelMap(m model.Metric, labelMap map[string]string, f includeFunc) {
	for ln, lv := range m {
		k := string(ln)
		v := string(lv)
		if f == nil || f(k, v) {
			common.AddToLabelMap(k, v, labelMap)
		}
	}
}

const (
	scale  = "scale"
	target = "target"
	ref    = "ref"
)

var (
	scaleTargetRef = common.JoinNoSep(scale, target, ref)
	strKind        = common.SnakeCase(scaleTargetRef, common.Kind)
	strName        = common.SnakeCase(scaleTargetRef, common.Name)
	hpaTargets     = []string{common.Deployment, common.StatefulSet, common.ReplicaSet, common.ReplicationController, common.Rollout}
)

func makeHpaWorkload() [][]model.SamplePair {
	return make([][]model.SamplePair, common.Params.Collection.HistoryInt)
}

func newHpa(obj *k8sObject, name, metricName, metricTargetType string, metricTargetValue float64) *hpa {
	h := &hpa{
		obj:               obj,
		name:              name,
		metricName:        metricName,
		metricTargetType:  metricTargetType,
		metricTargetValue: metricTargetValue,
		targetMetrics:     []*hpaTargetMetric{{Name: metricName, Type: metricTargetType, Value: metricTargetValue}},
		labels:            make(map[string]string),
	}
	if obj != nil {
		obj.hpa = h
	}
	return h
}

func (h *hpa) isClassified() bool {
	return h.obj != nil
}

func (h *hpa) addToLabelMap(ss *model.SampleStream) {
	addToLabelMap(ss.Metric, h.labels, excludeNodeLabel)
}

func (h *hpa) getGlobalMap() hpaMap {
	return hpaMaps[h.isClassified()]
}

func (h *hpa) addHpa(cluster, nsName, hpaValue string) {
	m := h.getGlobalMap()
	if m[cluster] == nil {
		m[cluster] = make(map[string]map[string]*hpa)
	}
	if m[cluster][nsName] == nil {
		m[cluster][nsName] = make(map[string]*hpa)
	}
	m[cluster][nsName][hpaValue] = h
}

func (h *hpa) merge(metricName, metricTargetType string, metricTargetValue float64) {
	var f bool
	for _, tm := range h.targetMetrics {
		if f = tm.Name == metricName; f {
			break
		}
	}
	h.targetMetrics = append(h.targetMetrics, &hpaTargetMetric{Name: metricName, Type: metricTargetType, Value: metricTargetValue})
	if !f {
		names := []string{h.metricName, metricName}
		h.metricName = common.Join(hpaSeparator, names...)
		// need to reset type and value
		h.metricTargetType = common.Empty
		h.metricTargetValue = common.UnknownValueFloat
	}
}

func findHpa(cluster, nsName, hpaName string) (h *hpa, ok bool) {
	for _, m := range hpaMaps {
		if h, ok = m[cluster][nsName][hpaName]; ok {
			break
		}
	}
	return
}

func (th *typeHolder) getHpa(cluster string, result model.Matrix) {
	for _, ss := range result {
		hpaLabel := th.getTypeLabelName()
		nsName, ns, values, ok := getNamespaceAndValues([]string{strKind, strName, hpaLabel, metricNameLabel, metricTargetTypeLabel}, cluster, ss)
		if !ok {
			continue
		}
		kind := strings.ToLower(values[strKind])
		name := values[strName]
		oid := &objectId{kind: kind, name: name}
		var obj *k8sObject
		if obj, ok = ns.objects[oid.Key(nsName)]; !ok {
			common.LogCluster(1, common.Warn, common.ClusterFormat+" failed to find HPA scale target reference of kind %s and name %s in namespace %s", cluster, true, cluster, kind, name, nsName)
			continue
		}
		metricTargetValue := common.LastValue(ss)
		// some custom HPA metrics coming from keda or Dynatrace may have commas and/or double quotes
		metricName := strings.ReplaceAll(strings.ReplaceAll(values[metricNameLabel], common.Comma, common.Underscore), common.DoubleQuote, common.Empty)
		metricTargetType := strings.ReplaceAll(strings.ReplaceAll(values[metricTargetTypeLabel], common.Comma, common.Underscore), common.DoubleQuote, common.Empty)
		var h *hpa
		if h, ok = findHpa(cluster, nsName, values[hpaLabel]); ok {
			h.merge(metricName, metricTargetType, metricTargetValue)
		} else {
			h = newHpa(obj, values[hpaLabel], metricName, metricTargetType, metricTargetValue)
			h.addHpa(cluster, nsName, values[hpaLabel])
		}
		h.addToLabelMap(ss)
	}
}

func (th *typeHolder) getHpaMetricString(cluster string, result model.Matrix) {
	for _, ss := range result {
		nsName, ns, hpaValue, ok := getNamespaceAndValue(th, cluster, ss)
		if !ok {
			continue
		}
		var h *hpa
		h, ok = findHpa(cluster, nsName, hpaValue)
		if !ok {
			// try to guess the HPA target, assuming the HPA will have the same name as the target
			oid := &objectId{name: hpaValue}
			for _, trg := range hpaTargets {
				oid.kind = trg
				var obj *k8sObject
				if obj, ok = ns.objects[oid.Key(nsName)]; ok {
					h = newHpa(obj, hpaValue, common.Empty, common.Empty, common.UnknownValueFloat)
					break
				}
			}
			if !ok {
				// no luck finding target
				h = newHpa(nil, hpaValue, common.Empty, common.Empty, common.UnknownValueFloat)
			}
			h.addHpa(cluster, nsName, hpaValue)
		}
		h.addToLabelMap(ss)
	}
}

var resourceHolder = &typeHolder{typeName: common.Resource}

func getNamespaceLimits(cluster string, result model.Matrix) {
	for _, ss := range result {
		_, ns, resource, ok := getNamespaceAndValue(resourceHolder, cluster, ss)
		if !ok {
			continue
		}
		value := common.LastValue(ss)
		switch resource {
		case common.RequestsCpu, common.Cpu:
			ns.cpuRequest = common.IntMCores(value)
		case common.LimitsCpu:
			ns.cpuLimit = common.IntMCores(value)
		case common.RequestsMem, common.Memory:
			ns.memRequest = common.IntMiB(value)
		case common.LimitsMem:
			ns.memLimit = common.IntMiB(value)
		case common.CountPods, common.Pods:
			ns.podsLimit = int(value)
		default:
		}
	}
}

func getNamespaceMetricString(cluster string, result model.Matrix) {
	for _, ss := range result {
		_, ns, _, ok := getNamespaceAndValue(nil, cluster, ss)
		if !ok {
			continue
		}
		for k, v := range ss.Metric {
			common.AddToLabelMap(string(k), string(v), ns.labelMap)
		}
	}
}

func newAggregatorWorkloadMetricHolder(aggregator string, aggregatorAsSuffix, workloadSuffix bool, metricName string) *common.WorkloadMetricHolder {
	var ne []string
	if aggregatorAsSuffix {
		ne = []string{metricName, aggregator}
	} else {
		ne = []string{aggregator, metricName}
	}
	wmh := common.NewWorkloadMetricHolder(ne...)
	if workloadSuffix {
		ne = append(ne, common.Workload)
		wmh = wmh.OverrideFileName(ne...)
	}
	return wmh
}

type workloadQuery struct {
	metricName         string
	baseQuery          string
	wqwIdx             int
	hasSuffix          bool
	aggregatorAsSuffix bool
	aggregators        map[string]string
	aggregatorNames    map[string]string
	groupClauses       map[string]*queryProcessorBuilder
}

func (th *typeHolder) containerFields(cluster string, fields []string) (cf []string, ok bool) {
	ctrIdx := containerIdx
	hasOwnerKind := th != nil && th.typeName != common.Empty
	if !hasOwnerKind {
		ctrIdx++
	}
	if len(fields) < ctrIdx+1 {
		return
	}
	for _, field := range fields {
		if field == common.Empty {
			return
		}
	}
	nsName := fields[nsIdx]
	var ns *namespace
	if ns, ok = namespaces[cluster][nsName]; !ok {
		return
	}
	var objKind string
	if hasOwnerKind {
		objKind = th.typeName
	} else {
		objKind = strings.ToLower(fields[ctrIdx-1])
	}
	var owner *k8sObject
	if owner, ok = getObject(cluster, nsName, objKind, fields[podIdx], ns, !hasOwnerKind); ok {
		if _, ok = owner.containers[fields[ctrIdx]]; ok {
			cf = append([]string{nsName, owner.name, getOwnerKindValue(owner.kind)}, fields[ctrIdx:]...)
		}
	}
	return
}

func getWorkload(wq *workloadQuery) {
	for aggregator, aggSuffix := range wq.aggregators {
		var agg string
		if agg, _ = wq.aggregatorNames[aggregator]; agg == common.Empty {
			agg = aggregator
		}
		wmh := newAggregatorWorkloadMetricHolder(agg, wq.aggregatorAsSuffix, wq.hasSuffix, wq.metricName)
		for _, lh := range labelHolders {
			queries := make(map[string]*common.QueryProcessor, len(wq.groupClauses))
			if lh.detected {
				q := wq.baseQuery + aggSuffix
				if wqw := lh.wqws[wq.wqwIdx]; wqw != nil {
					q = wqw.Wrap(q)
				}
				for groupClause, qpb := range wq.groupClauses {
					query := fmt.Sprintf("%s(%s%s", aggregator, q, groupClause)
					for i, ph := range labelPlaceholders {
						if i > 0 {
							query = strings.ReplaceAll(query, ph, lh.names[i])
						}
					}
					queries[query] = lh.getQueryProcessor(qpb)
				}
				wmh.GetWorkloadQueryVariants(1, queries, common.ContainerEntityKind)
			}
		}
	}
}

func hpaStatusConditionLabelFilter(lh *labelHolder) string {
	var st, cond interface{}
	if lh.names[podIdx] == common.Pod {
		st = true
		cond = scalingLimited
	} else {
		st = scalingLimited
		cond = true
	}
	return fmt.Sprintf(`{%s="%v", %s="%v"}`, status, st, condition, cond)
}

func hpaTargetMetricLabelFilter(metricName, metricTargetType string) string {
	return fmt.Sprintf(`{%s="%s", %s="%s"}`, metricNameLabel, metricName, metricTargetTypeLabel, metricTargetType)
}

type hpaWorkloadQuery struct {
	queryContext       string
	querySubject       []string
	metricNameSuffixes []string
}

var (
	hpaPrefix      = []string{common.Hpa}
	hpaExtraPrefix = []string{common.Hpa, common.Extra}
)

var targetMetricSubject = [2]string{target, common.Metric}

func (hwq *hpaWorkloadQuery) getWorkload(hmh *hpaMetricHolder, labelFilter string) {
	csvHeaderFormat, f := common.GetCsvHeaderFormat(common.HpaEntityKind, common.Metric)
	if !f {
		common.LogError(fmt.Errorf("no CSV header format found"), common.EntityFormat)
		return
	}
	// reset the workload for ALL HPAs
	for _, hMap := range hpaMaps {
		for _, cluster := range hMap {
			for _, ns := range cluster {
				for _, h := range ns {
					h.workload = makeHpaWorkload()
				}
			}
		}
	}
	mn := hpaPrefix
	xfn := hpaExtraPrefix
	if len(hwq.querySubject) != 2 || targetMetricSubject != ([2]string)(hwq.querySubject) {
		mn = append(mn, hwq.querySubject...)
		xfn = append(xfn, hwq.querySubject...)
	}
	mn = append(mn, hwq.metricNameSuffixes...)
	xfn = append(xfn, hwq.metricNameSuffixes...)
	swmh := common.NewWorkloadMetricHolder(mn...)
	xwmh := common.NewWorkloadMetricHolder(mn...).OverrideFileName(xfn...)
	wmhs := map[bool]*common.WorkloadMetricHolder{true: swmh, false: xwmh}
	l := len(wmhs)
	q := append([]string{hwq.queryContext}, hwq.querySubject...)
	query := hmh.query(q...) + labelFilter
	var foundValues map[string]bool
	for historyInterval := 0; historyInterval < common.Params.Collection.HistoryInt; historyInterval++ {
		range5Min := common.TimeRangeForInterval(time.Duration(historyInterval))
		if crm, _, err := common.CollectMetric(2, query, range5Min); err != nil {
			common.LogErrorWithLevel(1, common.Warn, err, common.QueryFormat, swmh.GetMetricName(), query)
		} else {
			foundValues = make(map[string]bool)
			for cluster, result := range crm {
				for _, ss := range result.Matrix {
					if nsName, _, hpaValue, ok := getNamespaceAndValue(hmh.typeHolder, cluster, ss); ok {
						var h *hpa
						if h, ok = findHpa(cluster, nsName, hpaValue); ok {
							h.workload[historyInterval] = append(h.workload[historyInterval], ss.Values...)
							if len(ss.Values) > 0 {
								foundValues[cluster] = true
							}
						}
					}
				}
			}
		}
	}
	if len(foundValues) == 0 {
		return
	}
	clusterFiles := make(map[string]map[bool]*os.File)
	for historyInterval := 0; historyInterval < common.Params.Collection.HistoryInt; historyInterval++ {
		for isClassified, hMap := range hpaMaps {
			for clName, cluster := range hMap {
				if !foundValues[clName] {
					continue
				}
				if clusterFiles[clName] == nil {
					clusterFiles[clName] = make(map[bool]*os.File, l)
				}
				wmh := wmhs[isClassified]
				clusterFiles[clName][isClassified] = common.InitWorkloadFile(clName, wmh.GetFileName(), hpaWorkloadEntityTypes[isClassified], csvHeaderFormat, wmh.GetMetricName())
				for nsName, ns := range cluster {
					for hpaName, h := range ns {
						var fieldSet [][]string
						if isClassified {
							for cName := range h.obj.containers {
								fieldSet = append(fieldSet, []string{nsName, h.obj.name, getOwnerKindValue(h.obj.kind), cName, hpaName})
							}
						} else {
							fieldSet = append(fieldSet, []string{nsName, common.Empty, common.Empty, common.Empty, hpaName})
						}
						for _, fields := range fieldSet {
							if err := common.WriteValues(clusterFiles[clName][isClassified], clName, common.JoinComma(fields...), h.workload[historyInterval], nil); err != nil {
								common.LogError(err, common.ClusterFileFormat, clName, wmhs[isClassified].GetFileName())
							}
						}
					}
				}
			}
		}
	}
	// close the workload files
	for cluster, files := range clusterFiles {
		for isClassified, file := range files {
			if file != nil {
				if err := file.Close(); err != nil {
					common.LogError(err, common.ClusterFileFormat, cluster, wmhs[isClassified].GetFileName())
				}
			}
		}
	}
}

type containerWorkloadProducer struct {
	cluster string
	nsName  string
	obj     *k8sObject
	c       *container
}

func (cwp *containerWorkloadProducer) GetCluster() string {
	return cwp.cluster
}

func (cwp *containerWorkloadProducer) GetEntityKind() string {
	return common.ContainerEntityKind
}

func (cwp *containerWorkloadProducer) GetRowPrefixes() (rps []string) {
	if cwp.c == nil {
		for cName := range cwp.obj.containers {
			rps = append(rps, cwp.getRowPrefix(cName))
		}
	} else {
		rps = append(rps, cwp.getRowPrefix(cwp.c.name))
	}
	return
}

func (cwp *containerWorkloadProducer) ShouldWrite(metric string) bool {
	if cwp.c == nil {
		return true
	}
	var f bool
	var w map[*container]bool
	if w, f = written[metric]; f {
		f = w[cwp.c]
	} else {
		w = make(map[*container]bool)
		written[metric] = w
	}
	w[cwp.c] = true
	return !f
}

func (cwp *containerWorkloadProducer) getRowPrefix(cName string) string {
	return fmt.Sprintf("%s,%s,%s,%s,%s", cwp.cluster, cwp.nsName, cwp.obj.name, getOwnerKindValue(cwp.obj.kind), common.ReplaceColons(cName))
}
