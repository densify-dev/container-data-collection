package container

import (
	"fmt"
	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/densify-dev/container-data-collection/internal/node"
	"github.com/prometheus/common/model"
	"strings"
	"sync"
	"time"
)

type namespace struct {
	objects                                               map[string]*k8sObject
	cpuLimit, cpuRequest, memLimit, memRequest, podsLimit int
	labelMap                                              map[string]string
}

type objectId struct {
	kind string
	name string
}

func (oid *objectId) String() string {
	return oid.Key(common.Empty)
}

func (oid *objectId) Key(nsName string) (s string) {
	if oid != nil {
		elements := []string{oid.kind, oid.name}
		if nsName != common.Empty {
			elements = append(elements, nsName)
		}
		s = key(elements...)
	}
	return
}

type ownership struct {
	directOwner   *objectId
	topLevelOwner *objectId
}

func newOwnership(kind, name string) *ownership {
	return &ownership{
		directOwner: &objectId{
			kind: strings.ToLower(kind),
			name: name,
		},
	}
}

func (o *ownership) getTopLevelOwner() (oid *objectId) {
	if o != nil {
		if oid = o.topLevelOwner; oid == nil {
			oid = o.directOwner
		}
	}
	return
}

type powerState int

const (
	terminated powerState = iota
	running
)

func (ps powerState) String() (s string) {
	switch ps {
	case terminated:
		s = common.Terminated
	case running:
		s = common.Running
	}
	return
}

type QosClass int

const (
	_ QosClass = iota
	bestEffort
	burstable
	guaranteed
)

const (
	BestEffort = "BestEffort"
	Burstable  = "Burstable"
	Guaranteed = "Guaranteed"
)

func (qc QosClass) String() (s string) {
	switch qc {
	case bestEffort:
		s = BestEffort
	case burstable:
		s = Burstable
	case guaranteed:
		s = Guaranteed
	default:
		s = common.Empty
	}
	return
}

var qosClassRanks = initQosClassRanks()

func initQosClassRanks() map[string]int {
	qcr := make(map[string]int, guaranteed)
	for qc := bestEffort; qc <= guaranteed; qc++ {
		qcr[qc.String()] = int(qc)
	}
	return qcr
}

func cmpQosClasses(qc1, qc2 string) int {
	return qosClassRanks[qc1] - qosClassRanks[qc2]
}

// k8sObject is used to hold information related to the highest owner of any containers
type k8sObject struct {
	*objectId
	containers  map[string]*container
	currentSize int
	createTime  time.Time
	labelMap    map[string]string
	hpa         *hpa
	qosClass    string
}

// container is used to hold information related to containers
type container struct {
	memory, gpuMemTotal, gpuMemCount,
	cpuLimit, cpuRequest,
	memLimit, memRequest,
	gpuLimit, gpuRequest,
	restarts,
	ephemeralStorageLimit, ephemeralStorageRequest int
	powerState                   powerState
	name                         string
	gpuModel, gpuSharingStrategy string
	labelMap                     map[string]string
}

var nonContinuousKinds = map[string]bool{
	common.Pod:         true,
	common.Job:         true,
	common.CronJob:     true,
	common.AnalysisRun: true,
}

func (obj *k8sObject) isRunningRelevant() bool {
	return obj != nil && nonContinuousKinds[obj.kind]
}

type clusterOwnerships map[string]*ownership

type hpaTargetMetric struct {
	Name  string  `json:"name"`
	Type  string  `json:"type"`
	Value float64 `json:"value"`
}

func (htm *hpaTargetMetric) String() (s string) {
	if htm == nil {
		s = common.Nil
	} else {
		s = fmt.Sprintf("%s%s%s%s%.3f", htm.Name, common.Or, htm.Type, common.Or, htm.Value)
	}
	return
}
func (htm *hpaTargetMetric) Order() (n int) {
	switch strings.ToLower(htm.Name) {
	case common.Memory:
		n = 2
	case common.Cpu:
		n = 1
	}
	return
}

func cmpHpaTargetMetric(htm1, htm2 *hpaTargetMetric) (n int) {
	if htm1 == nil {
		if htm2 != nil {
			n = -1
		}
	} else {
		if htm2 == nil {
			n = 1
		} else {
			n = htm1.Order() - htm2.Order()
		}
	}
	return
}

type hpa struct {
	obj               *k8sObject
	name              string
	metricName        string
	metricTargetType  string
	metricTargetValue float64
	labels            map[string]string
	targetMetrics     []*hpaTargetMetric
	workload          [][]model.SamplePair
}

type hpaMap map[string]map[string]map[string]*hpa

var (
	namespaces       = make(map[string]map[string]*namespace)
	ownerships       = make(map[string]clusterOwnerships)
	detectedOwners   = make(map[string]map[string]bool)
	objectHpas       = make(hpaMap)
	unclassifiedHpas = make(hpaMap)
	hpaMaps          = map[bool]hpaMap{true: objectHpas, false: unclassifiedHpas}
)

func key(s ...string) string {
	return common.Join(idSep, s...)
}

type includeFunc func(key, value string) bool

type typeHolder struct {
	typeName  string
	typeLabel string
}

func (th *typeHolder) getTypeLabelName() (ln string) {
	if th.typeLabel != common.Empty {
		ln = th.typeLabel
	} else {
		ln = th.typeName
	}
	return
}

func (th *typeHolder) getObjectId(ss *model.SampleStream) (oid *objectId) {
	if values, ok := th.getValues(ss); ok {
		oid = &objectId{kind: values[common.Kind], name: values[common.Name]}
	}
	return
}

func (th *typeHolder) getKey(ss *model.SampleStream) (k string) {
	if values, ok := th.getValues(ss); ok {
		k = key(values[common.Kind], values[common.Name], values[common.Namespace])
	}
	return
}

func (th *typeHolder) getValues(ss *model.SampleStream) (values map[string]string, ok bool) {
	typeLabelName := th.getTypeLabelName()
	if _, _, values, ok = getNamespaceAndValues([]string{typeLabelName}, common.Empty, ss); ok {
		values[common.Kind] = th.typeName
		values[common.Name] = values[typeLabelName]
	}
	return
}

func excludeNodeLabel(key, _ string) bool {
	return key != common.Node
}

// typeHolder vars
var (
	pth            = &typeHolder{typeName: common.Pod}
	dth            = &typeHolder{typeName: common.Deployment}
	rsth           = &typeHolder{typeName: common.ReplicaSet}
	rcth           = &typeHolder{typeName: common.ReplicationController}
	dsth           = &typeHolder{typeName: common.DaemonSet}
	ssth           = &typeHolder{typeName: common.StatefulSet}
	jth            = &typeHolder{typeName: common.Job, typeLabel: common.SnakeCase(common.Job, common.Name)}
	cjth           = &typeHolder{typeName: common.CronJob}
	hpath          = &typeHolder{typeName: common.Hpa, typeLabel: hpaFullName}
	oldhpath       = &typeHolder{typeName: common.Hpa}
	hpaTypeHolders = []*typeHolder{hpath, oldhpath}
)

var detectedOwnershipTypes = make(map[string]map[string]map[string]bool)

var ownerLabelNames = []string{ownerKind, ownerName}

func (th *typeHolder) getOwners(cluster string, result model.Matrix) {
	var co clusterOwnerships
	var f bool
	if co, f = ownerships[cluster]; !f {
		co = make(clusterOwnerships)
		ownerships[cluster] = co
	}
	var do map[string]bool
	if do, f = detectedOwners[cluster]; !f {
		do = make(map[string]bool)
		detectedOwners[cluster] = do
	}
	var ott map[string]map[string]bool
	if ott, f = detectedOwnershipTypes[th.typeName]; !f {
		ott = make(map[string]map[string]bool)
		detectedOwnershipTypes[th.typeName] = ott
	}
	for _, ss := range result {
		// don't pass cluster name as the cluster is not populated yet and we don't need the namespace
		if nsName, _, values, ok := getNamespaceAndValues(ownerLabelNames, common.Empty, ss); ok {
			owShip := newOwnership(values[ownerKind], values[ownerName])
			ownedId := th.getObjectId(ss)
			ownedKey := ownedId.Key(nsName)
			co[ownedKey] = owShip
			if isRelevant(cluster, nsName, ownedId) {
				do[owShip.directOwner.Key(nsName)] = true
				var cott map[string]bool
				if cott, f = ott[cluster]; !f {
					cott = make(map[string]bool)
					ott[cluster] = cott
				}
				cott[owShip.directOwner.kind] = true
			}
		}
	}
	if len(co) == 0 {
		common.LogCluster(1, common.Info, noOwnersFoundFormat, cluster, true, cluster, common.Plural(th.typeName))
	}
}

func addContainerAndOwners(cluster string, result model.Matrix) {
	var co clusterOwnerships
	var f bool
	if co, f = ownerships[cluster]; !f {
		err := fmt.Errorf("failed to find ownerships for cluster %s", cluster)
		common.LogError(err, "internal error")
		return
	}
	var cl map[string]*namespace
	if cl, f = namespaces[cluster]; !f {
		cl = make(map[string]*namespace)
		namespaces[cluster] = cl
	}
	for _, ss := range result {
		nsName, podName, containerName, ok := stdLabelHolder.values(ss)
		if !ok {
			continue
		}
		var ns *namespace
		if ns, f = cl[nsName]; !f {
			ns = &namespace{
				objects:    make(map[string]*k8sObject),
				cpuLimit:   common.UnknownValue,
				cpuRequest: common.UnknownValue,
				memLimit:   common.UnknownValue,
				memRequest: common.UnknownValue,
				podsLimit:  common.UnknownValue,
				labelMap:   make(map[string]string),
			}
			cl[nsName] = ns
		}
		var o *ownership
		podKey := key(common.Pod, podName, nsName)
		// get the direct owner (create one if there's none)
		if o, f = co[podKey]; f {
			var ows []*ownership
			// find the top-level owner
			for {
				if parent := co[key(o.directOwner.kind, o.directOwner.name, nsName)]; parent != nil && parent.directOwner.kind != common.Pod {
					ows = append(ows, o)
					o = parent
				} else {
					break
				}
			}
			for _, ow := range ows {
				ow.topLevelOwner = o.directOwner
			}
		} else {
			o = newOwnership(common.Pod, podName)
			co[podKey] = o
		}
		owner := o.getTopLevelOwner()
		ownerKey := owner.Key(nsName)
		var obj *k8sObject
		if obj, f = ns.objects[ownerKey]; !f {
			obj = &k8sObject{
				objectId:    owner,
				containers:  make(map[string]*container),
				currentSize: common.UnknownValue,
				labelMap:    make(map[string]string),
			}
			ns.objects[ownerKey] = obj
		}
		obj.containers[containerName] = &container{
			memory:                  common.UnknownValue,
			gpuMemTotal:             common.UnknownValue,
			gpuMemCount:             common.UnknownValue,
			cpuLimit:                common.UnknownValue,
			cpuRequest:              common.UnknownValue,
			memLimit:                common.UnknownValue,
			memRequest:              common.UnknownValue,
			gpuLimit:                common.UnknownValue,
			gpuRequest:              common.UnknownValue,
			powerState:              common.UnknownValue,
			ephemeralStorageLimit:   common.UnknownValue,
			ephemeralStorageRequest: common.UnknownValue,
			name:                    containerName,
			labelMap:                make(map[string]string),
		}
	}
}

func getOwnerQuery(metricName string, owned bool) (query string) {
	// Azure Monitor does not support PromQL regex properly -
	// https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/prometheus-api-promql#api-limitations
	// claims that "Query/series does not support regular expression filter".
	// In practice, it is supported, but with limitations;
	// e.g. regex with OR of an empty string like "<none>|" returns ALL label values i.s.o. "<none>" and the empty string only
	var binOp, logOp string
	if common.GetObservabilityPlatform() == common.AzureMonitorManagedPrometheus {
		if owned {
			binOp = "!="
			logOp = "and"
		} else {
			binOp = "="
			logOp = "or"
		}
		query = fmt.Sprintf(`%s{owner_name%s"<none>"} %s %s{owner_name%s""}`, metricName, binOp, logOp, metricName, binOp)
	} else {
		if owned {
			binOp = "!~"
		} else {
			binOp = "=~"
		}
		query = fmt.Sprintf(`%s{owner_name%s"<none>|"}`, metricName, binOp)
	}
	return
}

// Metrics function to collect data related to containers.
func Metrics() {
	var query string
	var err error
	var n int

	range5Min := common.TimeRange()

	common.DebugLogMemStats(1, "container data collection")
	// queries to gather hierarchy information for containers
	query = fmt.Sprintf(`sum(%s) by (namespace, pod, owner_name, owner_kind)`, getOwnerQuery("kube_pod_owner", true))
	if n, err = common.CollectAndProcessMetric(query, range5Min, pth.getOwners); err != nil || n == 0 {
		// error already handled
		return
	}
	query = fmt.Sprintf(`sum(%s) by (namespace, replicaset, owner_name, owner_kind)`, getOwnerQuery("kube_replicaset_owner", true))
	_, _ = common.CollectAndProcessMetric(query, range5Min, rsth.getOwners)
	query = fmt.Sprintf(`sum(%s) by (namespace, job_name, owner_name, owner_kind)`, getOwnerQuery("kube_job_owner", true))
	_, _ = common.CollectAndProcessMetric(query, range5Min, jth.getOwners)
	query = `max(kube_pod_container_info{}) by (container, pod, namespace)`
	if n, err = common.CollectAndProcessMetric(query, range5Min, addContainerAndOwners); err != nil || n == 0 {
		// error already handled
		return
	}

	// container metrics
	common.DebugLogObjectMemStats(common.Container)
	containerWorkloadWriters.AddMetricWorkloadWriters(common.CurrentSize, common.CpuLimits, common.CpuRequests, common.MemoryLimits, common.MemoryRequests, common.GpuLimits, common.GpuRequests)

	mh := &metricHolder{metric: common.Memory}
	query = `container_spec_memory_limit_bytes{name!~"k8s_POD_.*"}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getContainerMetric)

	if node.HasDcgmExporter(range5Min) {
		mh.metric = common.GpuMemoryTotal
		query = fmt.Sprintf("sum(%s) by (namespace, pod, container, %s, %s)", common.DcgmExporterLabelReplace("DCGM_FI_DEV_FB_USED{} + DCGM_FI_DEV_FB_FREE{}"), common.Node, common.ModelName)
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getContainerMetric)
	}

	stsq := fmt.Sprintf("sgn(sum(sum_over_time(kube_pod_container_info{}[%dm])) by (namespace,pod,container) - max(sum_over_time(kube_pod_container_status_terminated{}[%dm]) or sum_over_time(kube_pod_container_status_terminated_reason{}[%dm]) or sum_over_time(kube_pod_container_info{}[%dm])/100000) by (namespace,pod,container))",
		common.Params.Collection.SampleRate, common.Params.Collection.SampleRate, common.Params.Collection.SampleRate, common.Params.Collection.SampleRate)
	mh.metric = powerSt
	query = stsq
	_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getContainerMetric)

	fstsq := fmt.Sprintf(" unless on (namespace,pod,container) (%s == 0)", stsq)
	mh.metric = common.Limits
	query = fmt.Sprintf("sum(kube_pod_container_resource_limits{}%s) by (pod,namespace,container,resource)", fstsq)
	if n, err = common.CollectAndProcessMetric(query, range5Min, mh.getContainerMetric); err != nil || n < common.NumClusters() {
		mh.metric = common.CpuLimit
		query = fmt.Sprintf("sum(kube_pod_container_resource_limits_cpu_cores{}%s) by (pod,namespace,container)", fstsq)
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getContainerMetric)
		mh.metric = common.MemLimit
		query = fmt.Sprintf("sum(kube_pod_container_resource_limits_memory_bytes{}%s) by (pod,namespace,container)", fstsq)
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getContainerMetric)
	}
	mh.metric = common.Requests
	query = fmt.Sprintf("sum(kube_pod_container_resource_requests{}%s) by (pod,namespace,container,resource)", fstsq)
	if n, err = common.CollectAndProcessMetric(query, range5Min, mh.getContainerMetric); err != nil || n < common.NumClusters() {
		mh.metric = common.CpuRequest
		query = fmt.Sprintf("sum(kube_pod_container_resource_requests_cpu_cores{}%s) by (pod,namespace,container)", fstsq)
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getContainerMetric)
		mh.metric = common.MemRequest
		query = fmt.Sprintf("sum(kube_pod_container_resource_requests_memory_bytes{}%s) by (pod,namespace,container)", fstsq)
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getContainerMetric)
	}
	query = `kube_pod_container_info{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, getContainerMetricString)

	// pod metrics
	common.DebugLogObjectMemStats(common.Pod)
	query = `kube_pod_info{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, pth.getObjectMetricStringIncludeAll)
	query = `kube_pod_labels{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, pth.getObjectMetricString)

	mh.metric = restarts
	query = `sum(kube_pod_container_status_restarts_total{}) by (pod,namespace,container)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getContainerMetric)

	mh.metric = createTime
	omh := &objectMetricHolder{metricHolder: mh, typeHolder: pth}
	query = `kube_pod_created{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)

	mh.metric = qosClass
	query = `kube_pod_status_qos_class{} == 1`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)

	// namespace metrics
	common.DebugLogObjectMemStats(common.Namespace)
	query = `kube_namespace_labels{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, getNamespaceMetricString)
	query = `kube_namespace_annotations{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, getNamespaceMetricString)
	// this is min as want to know what the most restrictive quota is if there are multiple.
	query = `min(kube_resourcequota{type="hard"}) by (resource, namespace)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, getNamespaceLimits)

	// deployment metrics
	common.DebugLogObjectMemStats(common.Deployment)
	query = `kube_deployment_labels{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, dth.getObjectMetricString)

	omh.typeHolder = dth
	mh.metric = maxSurge
	query = `kube_deployment_spec_strategy_rollingupdate_max_surge{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)
	mh.metric = maxUnavailable
	query = `kube_deployment_spec_strategy_rollingupdate_max_unavailable{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)
	mh.metric = metadataGeneration
	query = `kube_deployment_metadata_generation{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)
	mh.metric = createTime
	query = `kube_deployment_created{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)

	// replicaset metrics
	common.DebugLogObjectMemStats(common.ReplicaSet)
	query = `kube_replicaset_labels{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, rsth.getObjectMetricString)
	omh.typeHolder = rsth
	query = `kube_replicaset_created{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)

	// replicationcontroller metrics
	common.DebugLogObjectMemStats(common.ReplicationController)
	query = `kube_replicationcontroller_created{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)

	// daemonset metrics
	common.DebugLogObjectMemStats(common.DaemonSet)
	query = `kube_daemonset_labels{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, dsth.getObjectMetricString)
	omh.typeHolder = dsth
	query = `kube_daemonset_created{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)

	// statefulset metrics
	common.DebugLogObjectMemStats(common.StatefulSet)
	query = `kube_statefulset_labels{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, ssth.getObjectMetricString)
	omh.typeHolder = ssth
	query = `kube_statefulset_created{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)

	// job metrics
	common.DebugLogObjectMemStats(common.Job)
	query = `kube_job_info{} * on (namespace,job_name) group_left (owner_name) max(kube_job_owner{}) by (namespace, job_name, owner_name)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, jth.getObjectMetricString)
	query = `kube_job_labels{} * on (namespace,job_name) group_left (owner_name) max(kube_job_owner{}) by (namespace, job_name, owner_name)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, jth.getObjectMetricString)
	omh.typeHolder = jth
	mh.metric = specCompletions
	query = `kube_job_spec_completions{} * on (namespace,job_name) group_left (owner_name) max(kube_job_owner{}) by (namespace, job_name, owner_name)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)
	mh.metric = specParallelism
	query = `kube_job_spec_parallelism{} * on (namespace,job_name) group_left (owner_name) max(kube_job_owner{}) by (namespace, job_name, owner_name)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)
	mh.metric = statusCompletionTime
	query = `kube_job_status_completion_time{} * on (namespace,job_name) group_left (owner_name) max(kube_job_owner{}) by (namespace, job_name, owner_name)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)
	mh.metric = statusStartTime
	query = `kube_job_status_start_time{} * on (namespace,job_name) group_left (owner_name) max(kube_job_owner{}) by (namespace, job_name, owner_name)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)
	mh.metric = createTime
	query = `kube_job_created{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)

	// cronjob metrics
	common.DebugLogObjectMemStats(common.CronJob)
	query = `kube_cronjob_labels{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, cjth.getObjectMetricString)
	query = `kube_cronjob_info{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, cjth.getObjectMetricString)
	omh.typeHolder = cjth
	mh.metric = nextScheduleTime
	query = `kube_cronjob_next_schedule_time{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)
	mh.metric = lastScheduleTime
	query = `kube_cronjob_status_last_schedule_time{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)
	mh.metric = statusActive
	query = `kube_cronjob_status_active{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)
	mh.metric = createTime
	query = `kube_cronjob_created{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)

	// HPA metrics
	common.DebugLogObjectMemStats(common.Hpa)
	var hmhs []*hpaMetricHolder
	var totals int
	for _, th := range hpaTypeHolders {
		hmh := &hpaMetricHolder{objectMetricHolder: &objectMetricHolder{typeHolder: th}}
		query = fmt.Sprintf("%s%s * on (namespace, %s) group_left (%s, %s) %s%s",
			hmh.query(spec, target, common.Metric), common.Braces, th.getTypeLabelName(),
			strKind, strName, hmh.query(common.InfoSt), common.Braces)
		_, _ = common.CollectAndProcessMetric(query, range5Min, hmh.getHpa)
		query = hmh.query(common.Labels) + common.Braces
		if n, err = common.CollectAndProcessMetric(query, range5Min, hmh.getHpaMetricString); n > 0 {
			hmhs = append(hmhs, hmh)
		}
		if totals += n; totals == common.NumClusters() {
			break
		}
	}

	// current size workloads
	common.DebugLogObjectMemStats(common.CurrentSizeName)
	mh.metric = common.CurrentSizeName
	omh.typeHolder = rsth
	oomh := &ownedObjectMetricHolder{objectMetricHolder: omh}
	query = `kube_replicaset_spec_replicas{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)
	query = `max(max(kube_replicaset_spec_replicas{}) by (namespace,replicaset) * on (namespace,replicaset) group_right max(kube_replicaset_owner{}) by (namespace, replicaset, owner_name)) by (owner_name, namespace)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, oomh.getObjectMetric)
	omh.typeHolder = rcth
	query = `kube_replicationcontroller_spec_replicas{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)
	omh.typeHolder = dsth
	query = `kube_daemonset_status_number_available{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)
	omh.typeHolder = ssth
	query = `kube_statefulset_replicas{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)
	omh.typeHolder = jth
	jczsq := fmt.Sprintf("max_over_time(kube_job_spec_parallelism{}[%dm]) and (max_over_time(kube_job_status_active{}[%dm]) == 1)", common.Params.Collection.SampleRate, common.Params.Collection.SampleRate)
	query = jczsq
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)
	query = fmt.Sprintf("max(max(%s) by (namespace,job_name) * on (namespace,job_name) group_right max(kube_job_owner{}) by (namespace, job_name, owner_name)) by (owner_name, namespace)", jczsq)
	_, _ = common.CollectAndProcessMetric(query, range5Min, oomh.getObjectMetric)

	containerWorkloadWriters.CloseAndClearWorkloadWriters(common.ContainerEntityKind)
	clear(written)

	writeConfig()
	writeAttributes()

	// container workloads
	common.DebugLogObjectMemStats(common.JoinSpace(common.Container, common.Workload))
	groupClauses := buildGroupClauses(common.Metric)
	wq := &workloadQuery{
		metricName:   common.CamelCase(common.Cpu, common.MCoresSt),
		baseQuery:    fmt.Sprintf(`round(max(irate(container_cpu_usage_seconds_total{name!~"k8s_POD_.*"}[%dm])) by (instance,%s,namespace,%s)*1000,1)`, common.Params.Collection.SampleRate, labelPlaceholders[podIdx], labelPlaceholders[containerIdx]),
		wqwIdx:       podIdx,
		hasSuffix:    true,
		aggregators:  map[string]string{common.Max: common.Empty, common.Avg: common.Empty},
		groupClauses: groupClauses,
	}
	getWorkload(wq)

	wq.metricName = common.Mem
	wq.aggregators[common.Avg] = " / (1024 * 1024)"
	wq.baseQuery = fmt.Sprintf(`max(container_memory_usage_bytes{name!~"k8s_POD_.*"}) by (instance,%s,namespace,%s)`, labelPlaceholders[podIdx], labelPlaceholders[containerIdx])
	getWorkload(wq)

	wq.metricName = rss
	wq.baseQuery = fmt.Sprintf(`max(container_memory_rss{name!~"k8s_POD_.*"}) by (instance,%s,namespace,%s)`, labelPlaceholders[podIdx], labelPlaceholders[containerIdx])
	getWorkload(wq)

	wq.metricName = common.WorkingSet
	wq.baseQuery = fmt.Sprintf(`sum(container_memory_working_set_bytes{name!~"k8s_POD_.*"}) by (instance,%s,namespace,%s)`, labelPlaceholders[podIdx], labelPlaceholders[containerIdx])
	getWorkload(wq)

	// container_fs_usage_bytes is an issue if the k8s cluster container runtime is containerd, see
	// https://github.com/google/cadvisor/issues/2785, https://github.com/google/cadvisor/issues/3315
	// it is supported by docker and cri-o container runtimes
	wq.metricName = common.Disk
	wq.aggregators[common.Avg] = common.Empty
	wq.baseQuery = fmt.Sprintf(`max(container_fs_usage_bytes{name!~"k8s_POD_.*"}) by (instance,%s,namespace,%s)`, labelPlaceholders[podIdx], labelPlaceholders[containerIdx])
	getWorkload(wq)

	wq.metricName = common.CamelCase(common.Cpu, common.Throttling, common.Percent)
	wq.baseQuery = fmt.Sprintf(`sum((round(increase(container_cpu_cfs_periods_total{name!~"k8s_POD_.*"}[%dm])) == 0) or (100 * round(increase(container_cpu_cfs_throttled_periods_total{name!~"k8s_POD_.*"}[%dm])) / round(increase(container_cpu_cfs_periods_total{name!~"k8s_POD_.*"}[%dm])))) by (instance,%s,namespace,%s)`,
		common.Params.Collection.SampleRate, common.Params.Collection.SampleRate, common.Params.Collection.SampleRate, labelPlaceholders[podIdx], labelPlaceholders[containerIdx])
	getWorkload(wq)

	wq.metricName = common.CamelCase(common.Cpu, common.Throttling, common.Seconds)
	wq.aggregators = map[string]string{common.Sum: common.Empty}
	wq.baseQuery = fmt.Sprintf(`sum(increase(container_cpu_cfs_throttled_seconds_total{name!~"k8s_POD_.*"}[%dm])) by (instance,%s,namespace,%s)`,
		common.Params.Collection.SampleRate, labelPlaceholders[podIdx], labelPlaceholders[containerIdx])
	getWorkload(wq)

	if node.HasDcgmExporter(range5Min) {
		getGpuWorkloads(wq)
	}

	wq.metricName = restarts
	wq.wqwIdx = containerIdx
	wq.hasSuffix = false
	wq.aggregators = map[string]string{common.Sum: common.Empty}
	wq.aggregatorNames = map[string]string{common.Sum: common.Max}
	wq.baseQuery = fmt.Sprintf(`max((round(increase(kube_pod_container_status_restarts_total{}[%dm]),1))%s) by (instance,pod,namespace,%s)`, common.Params.Collection.SampleRate, fstsq, labelPlaceholders[containerIdx])
	getWorkload(wq)

	spmxr := &hpaWorkloadQuery{
		queryContext: spec,
		querySubject: []string{common.Max, common.Replicas},
	}
	spmnr := &hpaWorkloadQuery{
		queryContext: spec,
		querySubject: []string{common.Min, common.Replicas},
	}
	stcr := &hpaWorkloadQuery{
		queryContext: status,
		querySubject: []string{common.Current, common.Replicas},
	}
	hwqs := []*hpaWorkloadQuery{spmxr, spmnr, stcr}

	targetMetricContexts := map[string]string{spec: target, status: common.Current}
	targetMetricQuerySubject := targetMetricSubject[:]
	targetMetricNames := []string{common.Cpu, common.Memory}
	targetMetricTypes := map[string]string{common.Avg: common.Average, common.Utilization: common.Utilization}
	for _, hmh := range hmhs {
		labelFilter := common.Braces
		var hwq *hpaWorkloadQuery
		for _, hwq = range hwqs {
			hwq.getWorkload(hmh, labelFilter)
		}
		hwq = &hpaWorkloadQuery{
			queryContext:       status,
			querySubject:       []string{condition},
			metricNameSuffixes: []string{scaling, limited},
		}
		for _, lh := range labelHolders {
			if lh.detected {
				labelFilter = hpaStatusConditionLabelFilter(lh)
				hwq.getWorkload(hmh, labelFilter)
			}
		}
		for ctx, c := range targetMetricContexts {
			for _, metricName := range targetMetricNames {
				for t, labelType := range targetMetricTypes {
					hwq = &hpaWorkloadQuery{
						queryContext:       ctx,
						querySubject:       targetMetricQuerySubject,
						metricNameSuffixes: []string{metricName, c, t},
					}
					labelFilter = hpaTargetMetricLabelFilter(metricName, labelType)
					hwq.getWorkload(hmh, labelFilter)
				}
			}
		}
	}
}

type gpuWorkloadQuery struct {
	metricName       string
	baseQuery        string
	appendToPrevious bool
}

var gpuWorkloadQueries = []*gpuWorkloadQuery{
	{
		metricName: common.CamelCase(common.Gpu, common.Utilization),
		baseQuery:  common.SafeDcgmGpuUtilizationQuery,
	},
	{
		metricName:       common.CamelCase(common.Gpu, common.Utilization, common.Gpus),
		baseQuery:        common.DcgmPercentQuerySuffix("kube_pod_container_resource_requests", common.Namespace, common.Pod, common.Container),
		appendToPrevious: true,
	},
	{
		metricName: common.CamelCase(common.Gpu, common.Mem, common.Utilization),
		baseQuery:  "100 * DCGM_FI_DEV_FB_USED{} / (DCGM_FI_DEV_FB_USED{} + DCGM_FI_DEV_FB_FREE{})",
	},
	{
		metricName: common.CamelCase(common.Gpu, common.Mem, common.Used),
		baseQuery:  "DCGM_FI_DEV_FB_USED{}",
	},
	{
		metricName: common.CamelCase(common.Gpu, common.Power, common.Usage),
		baseQuery:  "DCGM_FI_DEV_POWER_USAGE{}",
	},
}

var gpuAggregators = []string{common.Avg, common.Max}

func getGpuWorkloads(wq *workloadQuery) {
	wq.aggregatorAsSuffix = true
	for _, agg := range gpuAggregators {
		for _, gwq := range gpuWorkloadQueries {
			wq.metricName = gwq.metricName
			if gwq.appendToPrevious {
				wq.baseQuery += gwq.baseQuery
			} else {
				wq.baseQuery = common.DcgmAggOverTimeQuery(gwq.baseQuery, agg)
			}
			wq.aggregators = map[string]string{agg: common.Empty}
			getWorkload(wq)
		}
	}
	// restore the default
	wq.aggregatorAsSuffix = false
}

const (
	unownedGroupClause       = ` * on (namespace,pod) group_left max(%s) by (namespace,pod,%s)`
	directlyOwnedGroupClause = ` * on (namespace,pod) group_left (owner_kind,owner_name) max(kube_pod_owner{}) by (namespace,owner_kind,owner_name,pod)`
	replicaSetGroupClause    = ` * on (namespace,pod) group_left (replicaset) max(label_replace(kube_pod_owner{owner_kind="ReplicaSet"}, "replicaset", "$1", "owner_name", "(.*)")) by (namespace,replicaset,pod) * on (namespace,replicaset) group_left (owner_kind,owner_name) max(kube_replicaset_owner{}) by (namespace,owner_kind,owner_name,replicaset)`
	jobGroupClause           = ` * on (namespace,pod) group_left (job) max(label_replace(kube_pod_owner{owner_kind="Job"}, "job", "$1", "owner_name", "(.*)")) by (namespace,job,pod) * on (namespace,job) group_left (owner_kind,owner_name) max(label_replace(kube_job_owner{}, "job", "$1", "job_name", "(.*)")) by (namespace,owner_kind,owner_name,job)`
)

func buildGroupClauses(subject string) map[string]*queryProcessorBuilder {
	useSuffix := subject == common.Metric
	groupClauses := make(map[string]*queryProcessorBuilder, 4)
	buildGroupClause(groupClauses, false, unownedGroupClause, useSuffix)
	buildGroupClause(groupClauses, true, directlyOwnedGroupClause, useSuffix)
	if len(detectedOwnershipTypes[common.ReplicaSet]) > 0 {
		buildGroupClause(groupClauses, true, replicaSetGroupClause, useSuffix)
	}
	if len(detectedOwnershipTypes[common.Job]) > 0 {
		buildGroupClause(groupClauses, true, jobGroupClause, useSuffix)
	}
	return groupClauses
}

type groupClauseBuilder struct {
	suffix string
	args   []any
}

func buildGroupClause(qpbs map[string]*queryProcessorBuilder, owned bool, clauseFormat string, useSuffix bool) {
	ensureGroupClauseBuilders()
	gcb := groupClauseBuilders[owned]
	clf := clauseFormat
	var args []any
	if useSuffix {
		clf += gcb.suffix
		args = append(gcb.args, labelPlaceholders[containerIdx])
	} else {
		args = gcb.args
	}
	cl := fmt.Sprintf(clf, args...)
	var lnt labelNamesType
	var th *typeHolder
	if owned {
		lnt = fullOwnerLabelNames
		th = &typeHolder{}
	} else {
		lnt = podLabelNames
		th = pth
	}
	qpbs[cl] = &queryProcessorBuilder{lnt: lnt, th: th}
}

var groupClauseBuilders map[bool]*groupClauseBuilder

var gcbOnce sync.Once

func ensureGroupClauseBuilders() {
	gcbOnce.Do(func() {
		groupClauseBuilders = map[bool]*groupClauseBuilder{
			true:  {suffix: `) by (namespace,owner_kind,owner_name,%s)`},
			false: {suffix: `) by (namespace,pod,%s)`, args: []any{getOwnerQuery("kube_pod_owner", false), labelPlaceholders[containerIdx]}},
		}
	})
}
