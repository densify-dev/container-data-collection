package common

import (
	"context"
	"fmt"
	cconf "github.com/densify-dev/container-config/config"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/sigv4"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CollectMetric is used to query Prometheus to get data for specific query and return the results to be processed
func CollectMetric(callDepth int, query string, promRange *v1.Range) (crm ClusterResultMap, n int, err error) {
	cle := getClusterLabelsEmbedder(query)
	var qry string
	if qry, err = cle.embedClusterLabels(query); err != nil {
		return
	}
	pac := getApiCall(promRange)
	for _, qlf := range labelFilters {
		queries := qlf.adjustQuery(qry)
		for cluster, qr := range queries {
			if excludeQueryForCluster(cluster, qr) {
				logQuery(callDepth+1, cluster, qr+" - excluded for the cluster", pac)
				continue
			}
			q, si := adjustIntervalToScrapeInterval(cluster, qr)
			logQuery(callDepth+1, cluster, q, pac)
			var pa v1.API
			if pa, err = promApi(cluster); err == nil {
				ctx, cancel := context.WithCancel(context.Background())
				_ = time.AfterFunc(2*time.Minute, func() { cancel() })
				var value model.Value
				var e error
				switch pac {
				case ApiQuery:
					value, _, e = pa.Query(ctx, q, promRange.End)
				case ApiQueryRange:
					pr := adjustTimeRange(promRange, si)
					value, _, e = pa.QueryRange(ctx, q, *pr)
				case ApiQueryExemplars:
					// no use for exemplars yet, just for completeness
					_, e = pa.QueryExemplars(ctx, q, promRange.Start, promRange.End)
				}
				failOnConnectionError(e)
				m := qlf.filterValue(cluster, q, value, e)
				if crm, err = Merge(crm, m, Fail); err != nil {
					break
				}
			} else {
				failOnConnectionError(err)
			}
		}
	}
	for _, result := range crm {
		if result != nil && result.Matrix.Len() > 0 {
			n++
		}
	}
	return
}

func logQuery(callDepth int, cluster string, query string, pac PrometheusApiCall) {
	if cluster == Empty {
		LogAll(callDepth+1, Debug, queryLogFormat, pac, query)
	} else {
		LogCluster(callDepth+1, Debug, clusterQueryLogFormat, cluster, true, pac, cluster, query)
	}
}

func CheckPrometheusUp() (n int) {
	var err error
	var pa v1.API
	ctx, cancel := context.WithCancel(context.Background())
	_ = time.AfterFunc(2*time.Minute, func() { cancel() })
	if pa, err = promApi(Empty); err == nil {
		var value model.Value
		tr := TimeRange()
		if value, _, err = pa.QueryRange(ctx, "max(up)", *tr); err == nil {
			if mat, ok := value.(model.Matrix); ok {
				for _, ss := range mat {
					for _, v := range ss.Values {
						if v.Value > 0 {
							n++
						}
					}
				}
			}
		}
	}
	failOnConnectionError(err)
	return
}

func GetPrometheusVersion() (version string, found bool) {
	if supported, forWhat := buildInfoSupported(); !supported {
		version = fmt.Sprintf(verNotDetected, forWhat)
		return
	}
	var err error
	var pa v1.API
	ctx, cancel := context.WithCancel(context.Background())
	_ = time.AfterFunc(1*time.Minute, func() { cancel() })
	if pa, err = promApi(Empty); err == nil {
		var bir v1.BuildinfoResult
		if bir, err = pa.Buildinfo(ctx); err == nil {
			version = bir.Version
			found = true
		}
	}
	failOnConnectionError(err)
	return
}

var onceConn sync.Once

func failOnConnectionError(err error) {
	// if the very first attempt to connect to Prometheus fails, bail out as most probably
	// the configuration is wrong
	onceConn.Do(func() {
		if err != nil {
			FatalError(err, "Failed to connect to Prometheus:")
		}
	})
}

type ObservabilityPlatform string

const (
	UnknownPlatform               ObservabilityPlatform = Empty
	AWSManagedPrometheus          ObservabilityPlatform = "AWS Managed Prometheus"
	AzureMonitorManagedPrometheus ObservabilityPlatform = "Azure Monitor Managed Prometheus"
	GrafanaCloud                  ObservabilityPlatform = "Grafana Cloud"
)

const (
	workspaceAMPPattern = "aps-workspaces"
	fqdnAzMP            = "prometheus.monitor.azure.com"
	fqdnGrafanaCloud    = "grafana.net"
)

var op ObservabilityPlatform
var onceOp sync.Once

func GetObservabilityPlatform() ObservabilityPlatform {
	onceOp.Do(func() {
		op = getObservabilityPlatform()
	})
	return op
}

func getObservabilityPlatform() ObservabilityPlatform {
	host := strings.ToLower(Params.Prometheus.UrlConfig.Host)
	if Params.Prometheus.SigV4Config != nil || strings.HasPrefix(host, workspaceAMPPattern) {
		return AWSManagedPrometheus
	}
	if strings.HasSuffix(host, fqdnAzMP) {
		return AzureMonitorManagedPrometheus
	}
	if strings.Contains(host, fqdnGrafanaCloud) && Params.Prometheus.UrlConfig.Password != Empty {
		return GrafanaCloud
	}
	return UnknownPlatform
}

func buildInfoSupported() (bool, string) {
	switch observabilityPlatform := GetObservabilityPlatform(); observabilityPlatform {
	case AWSManagedPrometheus, AzureMonitorManagedPrometheus:
		// AMP and AzMP don't support Buildinfo() (both return 404):
		// * https://docs.aws.amazon.com/prometheus/latest/userguide/AMP-APIReference-Prometheus-Compatible-Apis.html
		// * https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/prometheus-api-promql#supported-apis
		// In addition, AWS SigV4 requires request body to sign (and BuildInfo is a GET with nil body), see
		// https://github.com/prometheus/common/issues/562
		return false, fmt.Sprintf(platformWorkspaces, observabilityPlatform)
	default:
		return true, Empty
	}
}

const (
	verNotDetected     = "cannot be detected for %s"
	platformWorkspaces = "%s workspaces"
	promClient         = "prometheus-client"
	labelPrefix        = Label + Underscore
)

func promApi(cluster string) (v1.API, error) {
	hcc := &config.HTTPClientConfig{}
	vop, err := cconf.NewValueOrPath(Params.Prometheus.CaCertPath, true, false)
	if err == nil {
		hcc.TLSConfig.CAFile = vop.Path()
	} else {
		FatalError(err, "failed to generate TLS config")
	}
	vop, _ = cconf.NewValueOrPath(Params.Prometheus.UrlConfig.Username, false, false)
	vop2, _ := cconf.NewValueOrPath(Params.Prometheus.UrlConfig.Password, false, false)
	if vop.IsEmpty() != vop2.IsEmpty() {
		FatalError(fmt.Errorf("basic auth requires both username and password"), "inconsistent configuration")
	}
	if !vop.IsEmpty() {
		hcc.BasicAuth = &config.BasicAuth{
			Username:     vop.Value(),
			UsernameFile: vop.Path(),
			Password:     config.Secret(vop2.Value()),
			PasswordFile: vop2.Path(),
		}
	}
	// Bearer token can be used for a number of solutions supporting Prometheus-API.
	// One of these is Azure Monitor managed Prometheus - see:
	// https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/prometheus-api-promql
	// Another one is Openshift Monitoring Stack - see:
	// https://docs.openshift.com/container-platform/4.15/monitoring/configuring-the-monitoring-stack.html
	// The bearer token can be passed as a string or as a path to a file.
	vop, err = cconf.NewValueOrPath(Params.Prometheus.BearerToken, false, false)
	if !vop.IsEmpty() {
		if vop.IsFile() {
			hcc.BearerTokenFile = vop.Path()
		} else {
			hcc.BearerToken = config.Secret(vop.Value())
		}
	}
	var rt http.RoundTripper
	if rt, err = config.NewRoundTripperFromConfig(*hcc, promClient); err != nil {
		FatalError(err, "failed to create HTTP round tripper")
	}
	if Params.Prometheus.SigV4Config != nil {
		if rt, err = sigv4.NewSigV4RoundTripper(Params.Prometheus.SigV4Config, rt); err != nil {
			FatalError(err, "failed to create AWS SigV4 round tripper")
		}
	}
	var hc *http.Client
	if hc, err = Params.Prometheus.RetryConfig.NewClient(rt, &ClusterLeveledLogger{cluster: cluster}); err != nil {
		return nil, err
	}
	var client api.Client
	if client, err = api.NewClient(api.Config{Address: Params.Prometheus.UrlConfig.Url, Client: hc}); err == nil {
		return v1.NewAPI(client), nil
	} else {
		return nil, err
	}
}

// TimeRange allows you to define the start and end values of the range will pass to the Prometheus for the query
func TimeRange() (promRange *v1.Range) {
	return TimeRangeForInterval(0)
}

func TimeRangeForInterval(historyInterval time.Duration) (promRange *v1.Range) {
	return TimeRangeForIntervals(historyInterval, 0, ApiQueryRange)
}

func TimeRangeEndTimeOnly() (promRange *v1.Range) {
	return TimeRangeForIntervals(0, 0, ApiQuery)
}

func TimeRangeForIntervals(historyInterval, absoluteStep time.Duration, target PrometheusApiCall) (promRange *v1.Range) {
	// for workload metrics the historyInterval will be set depending on how far back in history we are querying currently
	// note it will be 0 for all queries that are not workload related.
	end := CurrentTime.Add(-Interval * historyInterval)
	var start time.Time
	var step time.Duration
	switch target {
	default:
		// do nothing
	case ApiQueryRange:
		if absoluteStep > 0 {
			step = absoluteStep
		} else {
			step = Step
		}
		fallthrough
	case ApiQueryExemplars:
		start = end.Add(-Interval)
	}
	return &v1.Range{Start: start, End: end, Step: step}
}

type PrometheusApiCall uint

const (
	_ PrometheusApiCall = iota
	ApiQuery
	ApiQueryRange
	ApiQueryExemplars
)

func (pac PrometheusApiCall) String() string {
	switch pac {
	case ApiQuery:
		return "Query"
	case ApiQueryRange:
		return "QueryRange"
	case ApiQueryExemplars:
		return "QueryExemplars"
	default:
		return "unknown"
	}
}

func getApiCall(promRange *v1.Range) (pac PrometheusApiCall) {
	if promRange != nil {
		if promRange.Start.IsZero() {
			pac = ApiQuery
		} else {
			if promRange.Step == 0 {
				pac = ApiQueryExemplars
			} else {
				pac = ApiQueryRange
			}
		}
	}
	return
}

func adjustTimeRange(promRange *v1.Range, scrapeInterval time.Duration) (pr *v1.Range) {
	if promRange != nil && promRange.Step < time.Second && promRange.Step > 0 && scrapeInterval > 0 {
		// query resolution of less than a second doesn't make sense,
		// it is therefore a factor to multiply the scrape interval by
		pr = &v1.Range{Start: promRange.Start, End: promRange.End, Step: scrapeInterval * promRange.Step}
	} else {
		pr = promRange
	}
	return
}

var (
	invalidLabelCharRE = regexp.MustCompile(`[^a-zA-Z0-9_]`)
	matchAllCap        = regexp.MustCompile("([a-z0-9])([A-Z])")
)

// ToPrometheusLabelName is actually a copy of
// https://github.com/kubernetes/kube-state-metrics/blob/main/internal/store/utils.go#L125
// and added here to prevent dependency on that package for one function only;
// cannot use SnakeCase() as proper snake case treats a number as a new word (e.g. "group1" -> "group_1"),
// whereas ksm's toSnakeCase() does not (e.g. "group1" -> "group1"),
func ToPrometheusLabelName(s string) string {
	l := strings.ToLower(matchAllCap.ReplaceAllString(invalidLabelCharRE.ReplaceAllString(s, Underscore), "${1}_${2}"))
	if !strings.HasPrefix(l, labelPrefix) {
		l = labelPrefix + l
	}
	return l
}

func ToPrometheusLabelNameList(list string) string {
	orgNames := strings.Split(list, Comma)
	names := make([]string, len(orgNames))
	for i, orgName := range orgNames {
		names[i] = ToPrometheusLabelName(orgName)
	}
	return JoinComma(names...)
}

func CalculateScrapeIntervals() (err error) {
	et := TimeRangeEndTimeOnly()
	var query string
	for _, exporter := range exporters {
		var labelSelector string
		if len(exporter.repLabels) > 0 {
			labelSelector = Join(nonEmptyLabel+Comma, exporter.repLabels...) + nonEmptyLabel
		}
		query = fmt.Sprintf(`max(count_over_time(%s{%s}[%v])) by (job)`, exporter.repMetric, labelSelector, Interval)
		_, err = CollectAndProcessMetric(query, et, exporter.scrapeIntervalFromRepQuery)
	}
	query = fmt.Sprintf(`max(sum_over_time(up{}[%v])) by (job)`, Interval)
	if _, e := CollectAndProcessMetric(query, et, scrapeIntervalFromUp); err == nil && e != nil {
		err = e
	}
	for cluster, m := range clusterExporters {
		for _, ce := range m {
			LogCluster(1, Debug, ClusterFormat+" Prometheus exporter: %+v", cluster, true, cluster, ce)
		}
	}
	return
}

const (
	cadvisor     = "cadvisor"
	nodeExporter = "node-exporter"
	ksm          = "kube-state-metrics"
	ossm         = "openshift-state-metrics"
)

type exporter struct {
	name          string
	metricsPrefix string
	repMetric     string
	repLabels     []string
}

type clusterExporter struct {
	exporter
	promJob              string
	ActualScrapeInterval time.Duration // exported for fmt pretty-printing
	UpScrapeInterval     time.Duration // exported for fmt pretty-printing
}

func (e *exporter) scrapeIntervalFromRepQuery(cluster string, result model.Matrix) {
	l := len(exporters)
	if len(clusterExporters[cluster]) == 0 {
		clusterExporters[cluster] = make(map[string]*clusterExporter, l)
	}
	if len(clusterExportersByJob[cluster]) == 0 {
		clusterExportersByJob[cluster] = make(map[string][]*clusterExporter, l)
	}
	for _, ss := range result {
		if jobName := GetValue(ss, Job); jobName != Empty {
			ce := &clusterExporter{exporter: *e, promJob: jobName}
			setScrapeInterval(&ce.ActualScrapeInterval, ss)
			clusterExporters[cluster][e.metricsPrefix] = ce
			clusterExportersByJob[cluster][jobName] = append(clusterExportersByJob[cluster][jobName], ce)
		}
	}
}

func scrapeIntervalFromUp(cluster string, result model.Matrix) {
	for _, ss := range result {
		if jobName := GetValue(ss, Job); jobName != Empty {
			for _, ce := range clusterExportersByJob[cluster][jobName] {
				if ce != nil {
					setScrapeInterval(&ce.UpScrapeInterval, ss)
				}
			}
		}
	}
}

func setScrapeInterval(target *time.Duration, ss *model.SampleStream) {
	if len(ss.Values) > 0 {
		*target = (Interval / time.Duration(ss.Values[0].Value)).Round(time.Second)
	}
}

var exporters = makeExporters()

func makeExporters() []*exporter {
	exps := make([]*exporter, 0, 4)
	addExporter(&exps, cadvisor, "container_cpu_usage_seconds_total", []string{Container})
	addExporter(&exps, nodeExporter, "node_cpu_seconds_total", nil)
	addExporter(&exps, ksm, "kube_pod_info", nil)
	addExporter(&exps, ossm, "openshift_clusterresourcequota_usage", nil)
	return exps
}

func addExporter(exps *[]*exporter, name, repMetric string, repLabels []string) {
	*exps = append(*exps, &exporter{name: name, metricsPrefix: getExporterPrefix(repMetric), repMetric: repMetric, repLabels: repLabels})
}

var clusterExporters = make(map[string]map[string]*clusterExporter)
var clusterExportersByJob = make(map[string]map[string][]*clusterExporter)

const (
	rate     = "rate"
	increase = "increase"
	changes  = "changes"
)

var intervalFunctions = map[string]string{rate: "i", increase: Empty, changes: Empty}

func adjustIntervalToScrapeInterval(cluster string, query string) (q string, si time.Duration) {
	q = query
	if cluster == Empty {
		return
	}
	for f, prefixIgnore := range intervalFunctions {
		flb := f + leftBracket
		var prev string
		var reps []string
		for i, s := range strings.Split(q, flb) {
			var rep string
			if i > 0 && (prefixIgnore == Empty || !strings.HasSuffix(prev, prefixIgnore)) {
				if j := strings.Index(s, leftSquareBracket); j > -1 {
					if k := strings.Index(s[j:], rightSquareBracket); k > -1 {
						metricName := s[:j]
						scrapeInterval := getScrapeInterval(cluster, metricName)
						orgD := s[j+1 : j+k]
						var d time.Duration
						var err error
						if d, err = time.ParseDuration(orgD); err == nil {
							d += scrapeInterval
						} else {
							if strings.HasPrefix(orgD, Asterisk) {
								var n int64
								if n, err = strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(orgD, Asterisk)), 10, 64); err == nil {
									d = scrapeInterval * time.Duration(n)
								}
							}
						}
						if err == nil {
							rep = fmt.Sprintf("%s[%v]%s", metricName, d, s[j+k+1:])
							if si == 0 || scrapeInterval < si {
								si = scrapeInterval
							}
						}
					}
				}
			}
			if rep == Empty {
				rep = s
			}
			reps = append(reps, rep)
			prev = s
		}
		q = strings.Join(reps, flb)
	}
	return
}

func getScrapeInterval(cluster string, metricName string) (si time.Duration) {
	if e, f := clusterExporters[cluster][getExporterPrefix(metricName)]; f && e != nil {
		if e.ActualScrapeInterval > 0 {
			si = e.ActualScrapeInterval
		} else {
			si = e.UpScrapeInterval
		}
	}
	return
}

func getExporterPrefix(metricName string) string {
	return strings.Split(metricName, Underscore)[0]
}

type ClusterQueryExclusion func(cluster string, query string) bool

func RegisterClusterQueryExclusion(ce ClusterQueryExclusion) {
	clusterQueryExclusions = append(clusterQueryExclusions, ce)
}

var clusterQueryExclusions []ClusterQueryExclusion

func excludeQueryForCluster(cluster string, query string) bool {
	for _, ce := range clusterQueryExclusions {
		if ce(cluster, query) {
			return true
		}
	}
	return false
}

type ResolveMetricFunc func(cluster string, metricName string)
type ResolveMetricMap map[string]ResolveMetricFunc

var presentMetrics = make(map[string]map[string]bool)

func ResolveMetrics(m ResolveMetricMap) (err error) {
	et := TimeRangeEndTimeOnly()
	for metricName, f := range m {
		mr := &metricResolver{metricName: metricName, f: f}
		query := fmt.Sprintf(`max(present_over_time(%s{}[%v]))`, metricName, Interval)
		if _, err = CollectAndProcessMetric(query, et, mr.resolve); err != nil {
			break
		}
	}
	return
}

func IsMetricPresent(cluster, metricName string) bool {
	return presentMetrics[cluster][metricName]
}

type metricResolver struct {
	metricName string
	f          ResolveMetricFunc
}

func (mr *metricResolver) resolve(cluster string, result model.Matrix) {
	var clusterPresentMetrics map[string]bool
	var f bool
	if clusterPresentMetrics, f = presentMetrics[cluster]; !f {
		clusterPresentMetrics = make(map[string]bool)
		presentMetrics[cluster] = clusterPresentMetrics
	}
	if result.Len() > 0 {
		clusterPresentMetrics[mr.metricName] = true
		if mr.f != nil {
			mr.f(cluster, mr.metricName)
		}
	}
}
