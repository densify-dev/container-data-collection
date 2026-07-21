package container

import (
	"github.com/densify-dev/container-data-collection/internal/common"
)

const (
	idSep               = "__"
	powerSt             = "powerState"
	restart             = "restart"
	noOwnersFoundFormat = common.ClusterFormat + " - no %s owners found"
	create              = "create"
	surge               = "surge"
	unavailable         = "unavailable"
	metadata            = "metadata"
	generation          = "generation"
	spec                = "spec"
	status              = "status"
	completion          = "completion"
	parallelism         = "parallelism"
	start               = "start"
	next                = "next"
	last                = "last"
	schedule            = "schedule"
	active              = "active"
	horizontal          = "horizontal"
	autoscaler          = "autoscaler"
	kube                = "kube"
	rss                 = "rss"
	condition           = "condition"
	scaling             = "scaling"
	limited             = "limited"
	qos                 = "qos"
	class               = "class"
	hpaSeparator        = "###"
	kaiScheduler        = "kai-scheduler"
	id                  = "id"
	runtime             = "runtime"
	policy              = "policy"
	always              = "Always"
	runtimeLabel        = "runtime.kubex.ai"
)

var (
	restarts    = common.Plural(restart)
	hpaFullName = common.JoinNoSep(horizontal, common.Pod, autoscaler)
	// ownership labels
	ownerName = common.SnakeCase(common.Owner, common.Name)
	ownerKind = common.SnakeCase(common.Owner, common.Kind)
	// various
	createTime            = common.DromedaryCase(create, common.Time)
	maxSurge              = common.DromedaryCase(common.Max, surge)
	maxUnavailable        = common.DromedaryCase(common.Max, unavailable)
	metadataGeneration    = common.DromedaryCase(metadata, generation)
	specCompletions       = common.DromedaryCase(spec, common.Plural(completion))
	specParallelism       = common.DromedaryCase(spec, parallelism)
	statusCompletionTime  = common.DromedaryCase(status, completion, common.Time)
	statusStartTime       = common.DromedaryCase(status, start, common.Time)
	nextScheduleTime      = common.DromedaryCase(next, schedule, common.Time)
	lastScheduleTime      = common.DromedaryCase(last, schedule, common.Time)
	statusActive          = common.DromedaryCase(status, active)
	scalingLimited        = common.CamelCase(scaling, limited)
	metricNameLabel       = common.SnakeCase(common.Metric, common.Name)
	metricTargetTypeLabel = common.SnakeCase(common.Metric, target, common.Type)
	qosClass              = common.DromedaryCase(qos, class)
	qosClassLabel         = common.SnakeCase(qosClass)
	containerIdLabel      = common.SnakeCase(common.Container, id)
	restartPolicyLabel    = common.SnakeCase(restart, policy)
)
