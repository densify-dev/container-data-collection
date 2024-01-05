package common

import (
	cconf "github.com/densify-dev/container-config/config"
	"github.com/iancoleman/strcase"
	"strings"
)

// EntityKinds
const (
	ClusterEntityKind   = "cluster"
	NodeEntityKind      = Node
	ContainerEntityKind = Container
	CrqEntityKind       = "crq"
	RqEntityKind        = "rq"
	Quota               = "quota"
	Hpa                 = "hpa"
)

var (
	NodeGroupEntityKind = SnakeCase(Node, Group)
	HpaEntityKind       = SnakeCase(ContainerEntityKind, Hpa)
)

const (
	Empty       = cconf.Empty
	Cpu         = "cpu"
	Memory      = "memory"
	Mem         = "mem"
	Disk        = "disk"
	Net         = "net"
	Limit       = "limit"
	Request     = "request"
	Capacity    = "capacity"
	Allocatable = "allocatable"
	Resource    = "resource"
	Namespace   = "namespace"
	Type        = "type"
	Hard        = "hard"
	Used        = "used"
	Count       = "count"
	Node        = "node"
	Instance    = "instance"
	Speed       = "speed"
	Utilization = "utilization"
	Reservation = "reservation"
	Percent     = "percent"
	Workload    = "workload"
	Actual      = "actual"
	Raw         = "raw"
	Read        = "read"
	Write       = "write"
	Received    = "received"
	Sent        = "sent"
	Total       = "total"
	Bytes       = "bytes"
	Ops         = "ops"
	Packets     = "packets"
	Current     = "current"
	Size        = "size"
	Group       = "group"
	Name        = "name"
	Entity      = "entity"
	Metric      = "metric"
	Time        = "time"
	Pod         = "pod"
	Container   = "container"
	Replica     = "replica"
	Daemon      = "daemon"
	Stateful    = "stateful"
	Set         = "set"
	Job         = "job"
	Cron        = "cron"
	Deployment  = "deployment"
	Replication = "replication"
	Controller  = "controller"
	Owner       = "owner"
	Kind        = "kind"
	Running     = "Running"
	Terminated  = "Terminated"
	Day         = "day"
	Hour        = "hour"
	Label       = "label"
	Max         = "max"
	Avg         = "avg"
	Min         = "min"
	MCoresSt    = "mcores"
	Extra       = "extra"
	Bearer      = "Bearer"
	InfoSt      = "info"
	ConfigSt    = "config"
	Map         = "map"
	Catalog     = "catalog"
	Source      = "source"
)

// owner kind labels
var (
	PodOwner                   = CamelCase(Pod)
	DeploymentOwner            = CamelCase(Deployment)
	JobOwner                   = CamelCase(Job)
	NodeOwner                  = CamelCase(Node)
	ReplicaSetOwner            = CamelCase(Replica, Set)
	DaemonSetOwner             = CamelCase(Daemon, Set)
	StatefulSetOwner           = CamelCase(Stateful, Set)
	ReplicationControllerOwner = CamelCase(Replication, Controller)
	CronJobOwner               = CamelCase(Cron, Job)
	ConfigMapOwner             = CamelCase(ConfigSt, Map)   // openshift
	CatalogSourceOwner         = CamelCase(Catalog, Source) // openshift
)

// these are practically constants but as they use functions they need to be vars
var (
	Limits                = Plural(Limit)
	Requests              = Plural(Request)
	Pods                  = Plural(Pod)
	CpuLimit              = DromedaryCase(Cpu, Limit)
	LimitsCpu             = JoinDot(Limits, Cpu)
	MemLimit              = DromedaryCase(Memory, Limit)
	LimitsMem             = JoinDot(Limits, Memory)
	CpuRequest            = DromedaryCase(Cpu, Request)
	RequestsCpu           = JoinDot(Requests, Cpu)
	MemRequest            = DromedaryCase(Memory, Request)
	RequestsMem           = JoinDot(Requests, Memory)
	CpuCapacity           = DromedaryCase(Cpu, Capacity)
	MemCapacity           = DromedaryCase(Memory, Capacity)
	PodsCapacity          = DromedaryCase(Pods, Capacity)
	CpuAllocatable        = DromedaryCase(Cpu, Allocatable)
	MemAllocatable        = DromedaryCase(Memory, Allocatable)
	PodsAllocatable       = DromedaryCase(Pods, Allocatable)
	CountPods             = Join(Slash, Count, Pods)
	NetSpeedBytes         = DromedaryCase(Net, Speed, Bytes)
	ReplicaSet            = strings.ToLower(ReplicaSetOwner)
	DaemonSet             = strings.ToLower(DaemonSetOwner)
	StatefulSet           = strings.ToLower(StatefulSetOwner)
	ReplicationController = strings.ToLower(ReplicationControllerOwner)
	CronJob               = strings.ToLower(CronJobOwner)
	Replicas              = Plural(Replica)
	Days                  = Plural(Day)
	Hours                 = Plural(Hour)
	Labels                = Plural(Label)
	CurrentSizeName       = DromedaryCase(CurrentSize.GetMetricName())
	NodeGroupInclude      = JoinNoSep(Node, Group)
)

func Join(sep string, elements ...string) string {
	return strings.Join(elements, sep)
}

func JoinNoSep(elements ...string) string {
	return Join(Empty, elements...)
}

func JoinDot(elements ...string) string {
	return Join(Dot, elements...)
}

func JoinSpace(elements ...string) string {
	return Join(Space, elements...)
}

func JoinComma(elements ...string) string {
	return Join(Comma, elements...)
}

func CamelCase(elements ...string) string {
	return camelCase(JoinSpace(elements...))
}

func DromedaryCase(elements ...string) string {
	return dromedaryCase(JoinSpace(elements...))
}

func SnakeCase(elements ...string) string {
	return snakeCase(JoinSpace(elements...))
}

func Plural(s string) string {
	return s + "s"
}

const (
	exactEqual         = "="
	regexMatch         = "=~"
	Or                 = "|"
	doubleQuote        = "\""
	leftBrace          = "{"
	rightBrace         = "}"
	leftBracket        = "("
	rightBracket       = ")"
	Comma              = cconf.Comma
	Dot                = cconf.Dot
	Slash              = "/"
	Space              = " "
	Underscore         = "_"
	cr                 = "\r"
	lf                 = "\n"
	semicolonStr       = ";"
	semicolon          = ';'
	colon              = ":"
	emptyLabelSelector = leftBrace + rightBrace
	leftBraceComma     = leftBrace + Comma
	commaRightBrace    = Comma + rightBrace
	emptySelector      = leftBracket + rightBracket
	leftBracketComma   = leftBracket + Comma
	commaRightBracket  = Comma + rightBracket
	commaComma         = Comma + Comma
)

const (
	LabelNamesPlaceholder = `LNPH`
	// labelsPlaceholder contains - on purpose - characters which are invalid for model.LabelName
	labelsPlaceholder     = `#//#CLUSTER_LABELS#//#`
	emptyByClause         = Space + "by" + Space + emptySelector
	queryLogPrefix        = "QueryRange:"
	queryLogSuffix        = "query = %s"
	queryLogFormat        = queryLogPrefix + Space + queryLogSuffix
	clusterQueryLogFormat = queryLogPrefix + Space + ClusterFormat + Space + queryLogSuffix
	EntityFormat          = "entity=%s"
	ClusterFileFormat     = "cluster=%s file=%s"
)

const (
	maxKeyLen        = 250
	maxLabelValueLen = 255
)

func camelCase(s string) string {
	return strcase.ToCamel(s)
}

func dromedaryCase(s string) string {
	return strcase.ToLowerCamel(s)
}

func snakeCase(s string) string {
	return strcase.ToSnake(s)
}
