package container

import (
	"github.com/densify-dev/container-data-collection/internal/common"
)

const (
	idSep               = "__"
	powerSt             = "powerState"
	restarts            = "restarts"
	noOwnersFoundFormat = common.ClusterFormat + " - no %s owners found"
)

const (
	create      = "create"
	timeSt      = "time"
	surge       = "surge"
	unavailable = "unavailable"
	metadata    = "metadata"
	generation  = "generation"
	spec        = "spec"
	status      = "status"
	completion  = "completion"
	parallelism = "parallelism"
	start       = "start"
	next        = "next"
	last        = "last"
	schedule    = "schedule"
	active      = "active"
	horizontal  = "horizontal"
	autoscaler  = "autoscaler"
	kube        = "kube"
	rss         = "rss"
	workingSet  = "ws"
	condition   = "condition"
	scaling     = "scaling"
	limited     = "limited"
)

var (
	hpaFullName = common.JoinNoSep(horizontal, common.Pod, autoscaler)
	// ownership labels
	ownerName = common.SnakeCase(common.Owner, common.Name)
	ownerKind = common.SnakeCase(common.Owner, common.Kind)
	// various
	createTime            = common.DromedaryCase(create, timeSt)
	maxSurge              = common.DromedaryCase(common.Max, surge)
	maxUnavailable        = common.DromedaryCase(common.Max, unavailable)
	metadataGeneration    = common.DromedaryCase(metadata, generation)
	specCompletions       = common.DromedaryCase(spec, common.Plural(completion))
	specParallelism       = common.DromedaryCase(spec, parallelism)
	statusCompletionTime  = common.DromedaryCase(status, completion, timeSt)
	statusStartTime       = common.DromedaryCase(status, start, timeSt)
	nextScheduleTime      = common.DromedaryCase(next, schedule, timeSt)
	lastScheduleTime      = common.DromedaryCase(last, schedule, timeSt)
	statusActive          = common.DromedaryCase(status, active)
	scalingLimited        = common.CamelCase(scaling, limited)
	metricNameLabel       = common.SnakeCase(common.Metric, common.Name)
	metricTargetTypeLabel = common.SnakeCase(common.Metric, target, common.Type)
)
