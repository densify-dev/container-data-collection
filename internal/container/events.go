package container

import (
	"fmt"
	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/densify-dev/container-data-collection/internal/kubernetes"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"math"
	"strings"
	"time"
)

const (
	cadvisorOomKillsMetric            = `container_oom_events_total`
	ksmLastTerminatedTimestamp        = `kube_pod_container_status_last_terminated_timestamp`
	ksmLastTerminatedExitCode         = `kube_pod_container_status_last_terminated_exitcode`
	ksmRestartsTotal                  = `kube_pod_container_status_restarts_total`
	hiddenOomKillFraction             = 0.1371
	majorCgroupV2Grouping      uint64 = 1
	minorCgroupV2Grouping      uint64 = 28
	exitCodeFactor                    = 1000
	isPid1Factor                      = 10000
	residualFactor                    = isPid1Factor / exitCodeFactor
	fractionOffset                    = 0.5
)

func excludeHiddenOomKills(cluster string, query string) bool {
	return strings.Contains(query, cadvisorOomKillsMetric) &&
		kubernetes.HasMinimumVersion(cluster, majorCgroupV2Grouping, minorCgroupV2Grouping)
}

func excludeKsmLastTerminated(cluster string, query string) (result bool) {
	if result = strings.Contains(query, ksmLastTerminatedExitCode); result {
		result = common.IsMetricPresent(cluster, ksmLastTerminatedTimestamp) !=
			strings.Contains(query, ksmLastTerminatedTimestamp)
	}
	return
}

type ProcessExitEventProvider struct {
	PromRange *v1.Range
}

// TimeAndValues "parses" a Prometheus SamplePair (Timestamp and Value) and generates a time and values string from it.
// We need to extract the following data:
// - The timestamp (either the Prometheus timestamp OR embedded into the value)
// - The event count, typically 1 but may be higher (embedded into the value)
// - The exit code (embedded into the value)
// - Whether the process is PID 1 (embedded into the value)
//
// As the value is a single float64, we construct it as follows.
//  1. The integer part of the value is either the event count, or the timestamp multiplied by the event count
//     (if we have an exact timestamp as a metric value)
//  2. The fraction part of the value is constructed as follows:
//     a) As the exit code is in the range 0..255, we divide it by 1,000
//     b) As in most cases isPid1 is true, we take the numeric value of !isPid1 and divide it by 10,000;
//     so true yields 0 and false yields 0.0001
//     c) To avoid cases that math.Modf() will return values lesser than what we expect - which is possible due
//     to floating point precision - we add 0.5
//     d) The result is the sum of a), b) and c); it is in the range 0.5000..0.7551
//
// Parsing the value:
//  1. Use math.Modf() to separate the integer and fraction parts of the value
//  2. If the integer part is less than the minimal possible timestamp, it's assumed to be the event count,
//     and then the timestamp comes from the Prometheus SamplePair;
//  3. otherwise, it's the product of the timestamp and the event count and both are calculated from it
//  4. Subtract 0.5 from the fraction part and multiply it by 1,000
//  5. Round it to get the exit code
//  6. Subtract the exit code from the fraction part and take the absolute value
//  7. Multiply it by 10, round it and compare to 0 to get the isPid1 flag
func (peep *ProcessExitEventProvider) TimeAndValues(value *model.SamplePair) *common.TimeAndValues {
	var t model.Time
	// we need to distinguish between the cases when the integer part of the value is the event count,
	// and when it's the timestamp multiplied by the event count
	// it would suffice to subtract the scrape interval from the timestamp, but we don't have it here
	// - hence use the common step (which should be larger than the scrape interval)
	st := peep.PromRange.Start.Add(-common.Step).Unix()
	i, f := math.Modf(float64(value.Value))
	n := int64(math.Round(i))
	count := n / st
	if count == 0 {
		t = value.Timestamp
		count = n
	} else {
		t = model.TimeFromUnix(n / count)
	}
	f = (f - fractionOffset) * exitCodeFactor
	exitCode := math.Round(f)
	f = math.Abs(f - exitCode)
	isPid1 := math.Round(f*residualFactor) == 0
	return &common.TimeAndValues{
		Time:   t,
		Values: fmt.Sprintf("%d,%v", int(exitCode), isPid1),
		Count:  int(count),
	}
}

func (peep *ProcessExitEventProvider) CalculateRange(historyInterval int) *v1.Range {
	peep.PromRange = common.TimeRangeForIntervals(time.Duration(historyInterval), 1, common.ApiQueryRange)
	return peep.PromRange
}

func Events() {
	common.ResolveMetrics(map[string]common.ResolveMetricFunc{ksmLastTerminatedTimestamp: incrementIndicator})
	common.RegisterClusterQueryExclusion(excludeHiddenOomKills)
	common.RegisterClusterQueryExclusion(excludeKsmLastTerminated)
	multipliers := map[bool]string{true: common.Asterisk + ksmLastTerminatedTimestamp + common.Braces, false: common.Empty}
	eventQueries := make(map[string]int, 3)
	for _, f := range common.FoundIndicatorCounter(indicators, ksmLastTerminatedTimestamp) {
		eventQueries[makeRestartEventQuery(multipliers[f]).String()] = containerIdx
	}
	oomkeqb := &eventQueryBuilder{baseMetric: cadvisorOomKillsMetric + common.Braces, fraction: fmt.Sprintf(`%.4f`, hiddenOomKillFraction)}
	eventQueries[oomkeqb.String()] = podIdx
	groupClauses := buildGroupClauses(common.Event)
	getEvents(eventQueries, groupClauses)
}

type eventQueryBuilder struct {
	baseMetric string
	multiplier string
	fraction   string
}

const (
	metricFormat         = `(((round(increase(%s[*%d]) / %d) > 0)%s) + (%s) + %.1f)`
	scrapeIntervalFactor = 2
)

func (eqb *eventQueryBuilder) String() string {
	return fmt.Sprintf(metricFormat, eqb.baseMetric, scrapeIntervalFactor, scrapeIntervalFactor, eqb.multiplier, eqb.fraction, fractionOffset)
}

var exitCodeFraction = fmt.Sprintf(`(%s%s / %d)`, ksmLastTerminatedExitCode, common.Braces, exitCodeFactor)

func makeRestartEventQuery(multiplier string) *eventQueryBuilder {
	return &eventQueryBuilder{baseMetric: ksmRestartsTotal + common.Braces, multiplier: multiplier, fraction: exitCodeFraction}
}

var indicators = make(map[string]int)

func incrementIndicator(_ string, name string) {
	indicators[name] = indicators[name] + 1
}

const (
	eventMetricName = "ExitCode,IsPid1"
)

func getEvents(eventQueries map[string]int, groupClauses map[string]*queryProcessorBuilder) {
	for _, lh := range labelHolders {
		queries := make(map[string]*common.QueryProcessor, len(eventQueries)*len(groupClauses))
		if lh.detected {
			for baseQuery, wqwIdx := range eventQueries {
				q := baseQuery
				if wqw := lh.wqws[wqwIdx]; wqw != nil {
					q = wqw.Wrap(q)
				}
				for groupClause, qpb := range groupClauses {
					query := fmt.Sprintf("%s%s", q, groupClause)
					for i, ph := range labelPlaceholders {
						if i > 0 {
							query = strings.ReplaceAll(query, ph, lh.names[i])
						}
					}
					queries[query] = lh.getQueryProcessor(qpb)
				}
			}
			common.GetWorkloadQueryVariantsFieldConversion(1, common.Events, eventMetricName, queries, common.ContainerEntityKind, common.Event, &ProcessExitEventProvider{})
		}
	}
}
