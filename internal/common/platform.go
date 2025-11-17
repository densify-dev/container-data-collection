package common

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

type ObservabilityPlatform string

const (
	UnknownPlatform               ObservabilityPlatform = Empty
	AWSManagedPrometheus          ObservabilityPlatform = "AWS Managed Prometheus"
	AzureMonitorManagedPrometheus ObservabilityPlatform = "Azure Monitor Managed Prometheus"
	GoogleManagedPrometheus       ObservabilityPlatform = "Google Managed Prometheus"
	GrafanaCloud                  ObservabilityPlatform = "Grafana Cloud"
)

const (
	workspaceAMPPattern = "aps-workspaces"
	fqdnAzMP            = "prometheus.monitor.azure.com"
	domainGMP           = "monitoring.googleapis.com"
	fqdnGrafanaCloud    = "grafana.net"
)

var op ObservabilityPlatform
var opqa QueryAdjuster
var onceOp sync.Once

func GetObservabilityPlatform() ObservabilityPlatform {
	onceOp.Do(func() {
		op = getObservabilityPlatform()
		opqa = platformQueryAdjusters[op]
	})
	return op
}

func GetObservabilityPlatformQueryAdjuster() QueryAdjuster {
	_ = GetObservabilityPlatform()
	return opqa
}

func getObservabilityPlatform() ObservabilityPlatform {
	host := strings.ToLower(Params.Prometheus.UrlConfig.Host)
	if Params.Prometheus.SigV4Config != nil || strings.HasPrefix(host, workspaceAMPPattern) {
		return AWSManagedPrometheus
	}
	if strings.HasSuffix(host, fqdnAzMP) {
		return AzureMonitorManagedPrometheus
	}
	if strings.HasPrefix(host, domainGMP) {
		return GoogleManagedPrometheus
	}
	if strings.Contains(host, fqdnGrafanaCloud) && Params.Prometheus.UrlConfig.Password != Empty {
		return GrafanaCloud
	}
	return UnknownPlatform
}

const (
	// balancedParens pattern matches balanced parentheses. It can handle multiple levels of nesting.
	balancedParens = `\((?:[^()]*|\((?:[^()]*|\([^()]*\))*\))*\)`
	exported       = "exported"
	overTimeSuffix = "_over_time"
)

var (
	platformQueryAdjusters = map[ObservabilityPlatform]QueryAdjuster{GoogleManagedPrometheus: gmpQueryAdjuster}
	metricPrefixes         = []string{exporters[ksm].getPrefix(), exporters[dcgm].getPrefix()}
	exportedNamespace      = SnakeCase(exported, Namespace)
	exportedPod            = SnakeCase(exported, Pod)
	gmpRe                  = buildGmpRegex()
)

func nonCapturingGroup(s string) string {
	return fmt.Sprintf(`(?:%s)`, s)
}

func buildGmpRegex() *regexp.Regexp {
	// overTimePattern matches an `_over_time` and uses the balanced parentheses pattern
	// to correctly find the function's end.
	overTimePattern := fmt.Sprintf(`[a-zA-Z0-9_]+%s\s*%s`, overTimeSuffix, balancedParens)
	var patterns = make([]string, 0, len(metricPrefixes)+1)
	patterns = append(patterns, nonCapturingGroup(overTimePattern))
	for _, prefix := range metricPrefixes {
		metricPattern := fmt.Sprintf(`%s.+?\{[^}]*\}(\[.+?\])?`, prefix)
		patterns = append(patterns, nonCapturingGroup(metricPattern))
	}
	combinedPattern := strings.Join(patterns, "|")
	return regexp.MustCompile(combinedPattern)
}

// gmpQueryAdjuster finds all matches of the regex gmpKsmRe in the query,
// and applies double label replace prepends to each match, unless it's an over-time match
// with no kube-state-metrics metric
func gmpQueryAdjuster(query string) string {
	return gmpRe.ReplaceAllStringFunc(query, func(match string) string {
		// IMPORTANT CHECK: If we matched an `_over_time` function, we must double-check
		// that it actually contains a kube-state-metrics metric. This prevents false positives on
		// complex queries where the regex might over-match.
		if strings.HasSuffix(match, rightBracket) && strings.Contains(match, overTimeSuffix) {
			var found bool
			for _, metricPrefix := range metricPrefixes {
				if found = strings.Contains(match, metricPrefix); found {
					break
				}
			}
			if !found {
				// This was a faulty match (e.g., `max_over_time(node_exporter_metric{})`).
				// Return it unchanged.
				return match
			}
		}
		// For all valid matches (either the `_over_time` with a kube-state-metrics or DCGM metric,
		// or a raw kube-state-metrics or DCGM metric), apply the wrapper.
		return LabelReplace(LabelReplace(match, Namespace, exportedNamespace, HasValue), Pod, exportedPod, HasValue)
	})
}
