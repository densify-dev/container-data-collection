package container

import (
	"fmt"
	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/prometheus/common/model"
	"os"
	"strings"
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

// String - powerState is obtained using queries for Terminated status, therefore
// 0 means Running and 1 means Terminated
func (ps powerState) String() (s string) {
	switch ps {
	case 0:
		s = common.Running
	case 1:
		s = common.Terminated
	}
	return
}

// k8sObject is used to hold information related to the highest owner of any containers
type k8sObject struct {
	*objectId
	containers            map[string]*container
	currentSize, restarts int
	createTime            time.Time
	labelMap              map[string]string
}

// container is used to hold information related to containers
type container struct {
	memory, cpuLimit, cpuRequest, memLimit, memRequest, restarts int
	powerState                                                   powerState
	name                                                         string
	labelMap                                                     map[string]string
}

type clusterOwnerships map[string]*ownership

type hpa struct {
	obj      *k8sObject
	labels   map[string]string
	workload [][]model.SamplePair
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
	donth          = &typeHolder{typeName: common.Deployment, typeLabel: ownerName}
	rsth           = &typeHolder{typeName: common.ReplicaSet}
	rcth           = &typeHolder{typeName: common.ReplicationController}
	dsth           = &typeHolder{typeName: common.DaemonSet}
	ssth           = &typeHolder{typeName: common.StatefulSet}
	jth            = &typeHolder{typeName: common.Job, typeLabel: common.SnakeCase(common.Job, common.Name)}
	cjth           = &typeHolder{typeName: common.CronJob}
	cjonth         = &typeHolder{typeName: common.CronJob, typeLabel: ownerName}
	hpath          = &typeHolder{typeName: common.Hpa, typeLabel: hpaFullName}
	oldhpath       = &typeHolder{typeName: common.Hpa}
	hpaTypeHolders = []*typeHolder{hpath, oldhpath}
)

var detectedOwnerTypes = make(map[string]bool)

type ownedTypeHolder struct {
	ownedType *typeHolder
	ownerType string
}

var ownerLabelNames = []string{ownerKind, ownerName}

func (oth *ownedTypeHolder) getOwners(cluster string, result model.Matrix) {
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
	for _, ss := range result {
		// don't pass cluster name as the cluster is not populated yet and we don't need the namespace
		if nsName, _, values, ok := getNamespaceAndValues(ownerLabelNames, common.Empty, ss); ok {
			owShip := newOwnership(values[ownerKind], values[ownerName])
			ownedId := oth.ownedType.getObjectId(ss)
			ownedKey := ownedId.Key(nsName)
			co[ownedKey] = owShip
			if isRelevant(cluster, nsName, ownedId) {
				do[owShip.directOwner.Key(nsName)] = true
			}
		}
	}
	if oth.ownerType != common.Empty {
		if len(co) == 0 {
			common.LogCluster(1, common.Info, notFoundFormat, cluster, true, cluster, common.Plural(oth.ownerType))
		} else {
			detectedOwnerTypes[oth.ownerType] = true
		}
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
				restarts:    common.UnknownValue,
				labelMap:    make(map[string]string),
			}
			ns.objects[ownerKey] = obj
		}
		obj.containers[containerName] = &container{
			memory:     common.UnknownValue,
			cpuLimit:   common.UnknownValue,
			cpuRequest: common.UnknownValue,
			memLimit:   common.UnknownValue,
			memRequest: common.UnknownValue,
			restarts:   common.UnknownValue,
			powerState: common.UnknownValue,
			name:       containerName,
			labelMap:   make(map[string]string),
		}
	}
}

// Metrics function to collect data related to containers.
func Metrics() {
	var query string
	var err error
	var n int

	range5Min := common.TimeRange()

	common.DebugLogMemStats(1, "container data collection")
	// queries to gather hierarchy information for containers
	query = `sum(kube_pod_owner{owner_name!="<none>"}) by (namespace, pod, owner_name, owner_kind)`
	oth := &ownedTypeHolder{ownedType: pth}
	if n, err = common.CollectAndProcessMetric(query, range5Min, oth.getOwners); err != nil || n == 0 {
		// error already handled
		return
	}
	query = `sum(kube_replicaset_owner{owner_name!="<none>"}) by (namespace, replicaset, owner_name, owner_kind)`
	oth = &ownedTypeHolder{ownedType: rsth, ownerType: common.Deployment}
	_, _ = common.CollectAndProcessMetric(query, range5Min, oth.getOwners)
	query = `sum(kube_job_owner{owner_name!="<none>"}) by (namespace, job_name, owner_name, owner_kind)`
	oth = &ownedTypeHolder{ownedType: jth, ownerType: common.CronJob}
	_, _ = common.CollectAndProcessMetric(query, range5Min, oth.getOwners)
	query = `max(kube_pod_container_info{}) by (container, pod, namespace)`
	if n, err = common.CollectAndProcessMetric(query, range5Min, addContainerAndOwners); err != nil || n == 0 {
		// error already handled
		return
	}

	// container metrics
	common.DebugLogObjectMemStats(common.Container)
	mh := &metricHolder{metric: common.Memory}
	query = `container_spec_memory_limit_bytes{name!~"k8s_POD_.*"}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getContainerMetric)
	mh.metric = common.Limits
	query = `sum(kube_pod_container_resource_limits{}) by (pod,namespace,container,resource)`
	if n, err = common.CollectAndProcessMetric(query, range5Min, mh.getContainerMetric); err != nil || n < common.NumClusters() {
		mh.metric = common.CpuLimit
		query = `sum(kube_pod_container_resource_limits_cpu_cores{}) by (pod,namespace,container)`
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getContainerMetric)
		mh.metric = common.MemLimit
		query = `sum(kube_pod_container_resource_limits_memory_bytes{}) by (pod,namespace,container)`
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getContainerMetric)
	}
	mh.metric = common.Requests
	query = `sum(kube_pod_container_resource_requests{}) by (pod,namespace,container,resource)`
	if n, err = common.CollectAndProcessMetric(query, range5Min, mh.getContainerMetric); err != nil || n < common.NumClusters() {
		mh.metric = common.CpuRequest
		query = `sum(kube_pod_container_resource_requests_cpu_cores{}) by (pod,namespace,container)`
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getContainerMetric)
		mh.metric = common.MemRequest
		query = `sum(kube_pod_container_resource_requests_memory_bytes{}) by (pod,namespace,container)`
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

	mh.metric = powerSt
	query = `sum(kube_pod_container_status_terminated{}) by (pod,namespace,container)`
	if n, err = common.CollectAndProcessMetric(query, range5Min, mh.getContainerMetric); err != nil || n < common.NumClusters() {
		query = `sum(kube_pod_container_status_terminated_reason{}) by (pod,namespace,container)`
		_, _ = common.CollectAndProcessMetric(query, range5Min, mh.getContainerMetric)
	}

	mh.metric = createTime
	omh := &objectMetricHolder{metricHolder: mh, typeHolder: pth}
	query = `kube_pod_created{}`
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
		hmh := &hpaMetricHolder{objectMetricHolder: &objectMetricHolder{}}
		hmh.typeHolder = th
		query = hmh.query(common.InfoSt)
		_, _ = common.CollectAndProcessMetric(query, range5Min, hmh.getHpa)
		query = hmh.query(common.Labels)
		if n, err = common.CollectAndProcessMetric(query, range5Min, hmh.getHpaMetricString); n > 0 {
			hmhs = append(hmhs, hmh)
		}
		if totals += n; totals == common.NumClusters() {
			break
		}
	}

	// current size workloads
	common.DebugLogObjectMemStats(common.CurrentSizeName)
	objWorkloadWriters[common.CurrentSizeName] = make(map[string]*os.File)
	mh.metric = common.CurrentSizeName
	omh.typeHolder = rsth
	query = `kube_replicaset_spec_replicas{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)
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
	query = `kube_job_spec_parallelism{}`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)
	omh.typeHolder = cjonth
	query = `max(max(kube_job_spec_parallelism{}) by (namespace,job_name) * on (namespace,job_name) group_right max(kube_job_owner{}) by (namespace, job_name, owner_name)) by (owner_name, namespace)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)
	omh.typeHolder = donth
	query = `max(max(kube_replicaset_spec_replicas{}) by (namespace,replicaset) * on (namespace,replicaset) group_right max(kube_replicaset_owner{}) by (namespace, replicaset, owner_name)) by (owner_name, namespace)`
	_, _ = common.CollectAndProcessMetric(query, range5Min, omh.getObjectMetric)

	for cluster, file := range objWorkloadWriters[common.CurrentSizeName] {
		if err = file.Close(); err != nil {
			common.LogError(err, common.DefaultLogFormat, cluster, common.ContainerEntityKind)
		}
	}
	delete(objWorkloadWriters, common.CurrentSizeName)

	writeConfig()
	writeAttributes()

	// container workloads
	common.DebugLogObjectMemStats(common.JoinSpace(common.Container, common.Workload))
	groupClauses := map[string]*queryProcessorBuilder{
		fmt.Sprintf(` * on (pod, namespace) group_left max(kube_pod_owner{owner_name=~"<none>|"}) by (namespace, pod, %s)) by (pod,namespace,%s)`, labelPlaceholders[containerIdx], labelPlaceholders[containerIdx]):         {lnt: podLabelNames, th: pth},
		fmt.Sprintf(` * on (pod, namespace) group_left (owner_name,owner_kind) max(kube_pod_owner{}) by (namespace, pod, owner_name, owner_kind)) by (owner_kind,owner_name,namespace,%s)`, labelPlaceholders[containerIdx]): {lnt: fullOwnerLabelNames, th: &typeHolder{}},
	}
	if detectedOwnerTypes[common.Deployment] == true {
		grClauseDeployment := fmt.Sprintf(` * on (pod, namespace) group_left (replicaset) max(label_replace(kube_pod_owner{owner_kind="ReplicaSet"}, "replicaset", "$1", "owner_name", "(.*)")) by (namespace, pod, replicaset) * on (replicaset, namespace) group_left (owner_name) max(kube_replicaset_owner{owner_kind="Deployment"}) by (namespace, replicaset, owner_name)) by (owner_name,namespace,%s)`, labelPlaceholders[containerIdx])
		groupClauses[grClauseDeployment] = &queryProcessorBuilder{
			lnt: typeOwnerLabelNames,
			th:  donth,
		}
	}
	if detectedOwnerTypes[common.CronJob] == true {
		grClauseCronJob := fmt.Sprintf(` * on (pod, namespace) group_left (job) max(label_replace(kube_pod_owner{owner_kind="Job"}, "job", "$1", "owner_name", "(.*)")) by (namespace, pod, job) * on (job, namespace) group_left (owner_name) max(label_replace(kube_job_owner{owner_kind="CronJob"}, "job", "$1", "job_name", "(.*)")) by (namespace, job, owner_name)) by (owner_name,namespace,%s)`, labelPlaceholders[containerIdx])
		groupClauses[grClauseCronJob] = &queryProcessorBuilder{
			lnt: typeOwnerLabelNames,
			th:  cjonth,
		}
	}
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

	wq.metricName = workingSet
	wq.baseQuery = fmt.Sprintf(`max(container_memory_working_set_bytes{name!~"k8s_POD_.*"}) by (instance,%s,namespace,%s)`, labelPlaceholders[podIdx], labelPlaceholders[containerIdx])
	getWorkload(wq)

	// container_fs_usage_bytes is an issue if the k8s cluster container runtime is containerd, see
	// https://github.com/google/cadvisor/issues/2785, https://github.com/google/cadvisor/issues/3315
	// it is supported by docker and cri-o container runtimes
	wq.metricName = common.Disk
	wq.aggregators[common.Avg] = common.Empty
	wq.baseQuery = fmt.Sprintf(`max(container_fs_usage_bytes{name!~"k8s_POD_.*"}) by (instance,%s,namespace,%s)`, labelPlaceholders[podIdx], labelPlaceholders[containerIdx])
	getWorkload(wq)

	wq.metricName = restarts
	wq.wqwIdx = containerIdx
	wq.hasSuffix = false
	wq.aggregators = map[string]string{common.Max: common.Empty}
	wq.baseQuery = fmt.Sprintf(`max(round(increase(kube_pod_container_status_restarts_total{name!~"k8s_POD_.*"}[%dm]),1)) by (instance,pod,namespace,%s)`, common.Params.Collection.SampleRate, labelPlaceholders[containerIdx])
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
	for _, hmh := range hmhs {
		var clause string
		var hwq *hpaWorkloadQuery
		for _, hwq = range hwqs {
			hwq.getWorkload(hmh, clause)
		}
		hwq = &hpaWorkloadQuery{
			queryContext:       status,
			querySubject:       []string{condition},
			metricNameSuffixes: []string{scaling, limited},
		}
		for _, lh := range labelHolders {
			if lh.detected {
				clause = hpaStatusConditionClause(lh)
				hwq.getWorkload(hmh, clause)
			}
		}
	}
}
