package common

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	cconf "github.com/densify-dev/container-config/config"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/sigv4"
)

// CollectMetric is used to query Prometheus to get data for specific query and return the results to be processed
func CollectMetric(callDepth int, query string, promRange *v1.Range) (crm ClusterResultMap, n int, err error) {
	var qry string
	if pqa := GetObservabilityPlatformQueryAdjuster(); pqa == nil {
		qry = query
	} else {
		qry = pqa(query)
	}
	cle := getClusterLabelsEmbedder(qry)
	if qry, err = cle.embedClusterLabels(qry); err != nil {
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

func LogPrometheusTsdbStatus() (err error) {
	if !Params.Debug || GetObservabilityPlatform() != UnknownPlatform {
		return
	}
	var pa v1.API
	if pa, err = promApi(Empty); err == nil {
		ctx, cancel := context.WithCancel(context.Background())
		_ = time.AfterFunc(1*time.Minute, func() { cancel() })
		var tsdbResult v1.TSDBResult
		if tsdbResult, err = pa.TSDB(ctx); err == nil {
			var b []byte
			if b, err = json.Marshal(&tsdbResult); err == nil {
				LogAll(1, Debug, "Prometheus TSDB status: %s", string(b))
			}
		}
	}
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

func buildInfoSupported() (bool, string) {
	switch observabilityPlatform := GetObservabilityPlatform(); observabilityPlatform {
	case AWSManagedPrometheus, AzureMonitorManagedPrometheus, GoogleManagedPrometheus:
		// AMP, AzMP and GMP don't support Buildinfo() (all return 404):
		// * https://docs.aws.amazon.com/prometheus/latest/userguide/AMP-APIReference-Prometheus-Compatible-Apis.html
		// * https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/prometheus-api-promql#supported-apis
		// * https://docs.cloud.google.com/stackdriver/docs/managed-prometheus/query-api-ui#http-api-details
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
	// Another one is Google Managed Prometheus - see:
	// https://docs.cloud.google.com/stackdriver/docs/managed-prometheus/query-api-ui#api-prometheus
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
	for _, exp := range exporters {
		var labelSelector string
		if len(exp.repLabels) > 0 {
			labelSelector = Join(nonEmptyLabel+Comma, exp.repLabels...) + nonEmptyLabel
		}
		query = fmt.Sprintf(`max(count_over_time(%s{%s}[%v])) by (job)`, exp.repMetric, labelSelector, Interval)
		_, err = CollectAndProcessMetric(query, et, exp.scrapeIntervalFromRepQuery)
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
	prometheusMetricName = "__name__"
	metricName           = "metric_name"
	allMetricsQueryFmt   = `{%s=~"%s_%s"}`
	allMetricsFmt        = `group by (%s) (%s)`
)

func LogAllMetrics() (err error) {
	et := TimeRangeEndTimeOnly()
	var query string
	for _, exp := range exporters {
		if exp.logAllMetrics || Params.Debug {
			query = fmt.Sprintf(allMetricsQueryFmt, prometheusMetricName, exp.metricsPrefix, Always.String())
			query = aggOverTimeQuery(query, Last, Interval)
			query = LabelReplace(query, metricName, prometheusMetricName, HasValue)
			query = fmt.Sprintf(allMetricsFmt, metricName, query)
			_, err = CollectAndProcessMetric(query, et, exp.logAllClusterMetrics)
		}
	}
	return
}

const (
	cadvisor         = "cadvisor"
	nodeExporter     = "node-exporter"
	ksm              = "kube-state-metrics"
	ossm             = "openshift-state-metrics"
	Dcgm             = "dcgm-exporter"
	ephemeralStorage = "k8s-ephemeral-storage-metrics"
	KubexGpu         = "kubex-gpu-process-exporter"
	Beyla            = "beyla"
)

type exporter struct {
	name          string
	metricsPrefix string
	repMetric     string
	repLabels     []string
	logAllMetrics bool
}

type clusterExporter struct {
	exporter
	promJob              string
	ActualScrapeInterval time.Duration // exported for fmt pretty-printing
	UpScrapeInterval     time.Duration // exported for fmt pretty-printing
}

func (e *exporter) getPrefix() string {
	return e.metricsPrefix + Underscore
}

func (e *exporter) logAllClusterMetrics(cluster string, result model.Matrix) {
	s := make([]string, 0, result.Len())
	for _, ss := range result {
		if mn, f := GetLabelValue(ss, metricName); f {
			s = append(s, mn)
		}
	}
	sort.Strings(s)
	var level LogLevel
	if e.logAllMetrics {
		level = Info
	}
	LogCluster(1, level, ClusterFormat+" exporter=%s detected metrics=%v", cluster, true, cluster, e.name, s)
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

func makeExporters() map[string]*exporter {
	exps := make(map[string]*exporter, 7)
	addExporter(exps, cadvisor, "container_cpu_usage_seconds_total", []string{Container}, false)
	addExporter(exps, nodeExporter, "node_cpu_seconds_total", nil, false)
	addExporter(exps, ksm, "kube_pod_info", nil, false)
	addExporter(exps, ossm, "openshift_clusterresourcequota_usage", nil, false)
	addExporter(exps, Dcgm, "DCGM_FI_DEV_GPU_UTIL", nil, true)
	addExporter(exps, ephemeralStorage, "ephemeral_storage_node_available", nil, true)
	addExporter(exps, KubexGpu, "kubex_gpu_container_requests", nil, true)
	addExporter(exps, Beyla, SurveyInfo, nil, true)
	return exps
}

func addExporter(exps map[string]*exporter, name, repMetric string, repLabels []string, logAllMetrics bool) {
	exps[name] = &exporter{name: name, metricsPrefix: getExporterPrefix(repMetric), repMetric: repMetric, repLabels: repLabels, logAllMetrics: logAllMetrics}
}

var clusterExporters = make(map[string]map[string]*clusterExporter)
var clusterExportersByJob = make(map[string]map[string][]*clusterExporter)

type intervalFunction string

const (
	rate          intervalFunction = "rate"
	irate         intervalFunction = "irate"
	increase      intervalFunction = "increase"
	changes       intervalFunction = "changes"
	delta         intervalFunction = "delta"
	idelta        intervalFunction = "idelta"
	deriv         intervalFunction = "deriv"
	predictLinear intervalFunction = "predict_linear"
	holtWinters   intervalFunction = "holt_winters"
	resets        intervalFunction = "resets"
)

var intervalFunctions = []intervalFunction{rate, irate, increase, changes, intervalFunction(OverTimeSuffix), delta, idelta, deriv, predictLinear, holtWinters, resets}

func scrapeIntervalMultiplication(scrapeInterval time.Duration, dur string) (d time.Duration, err error) {
	if !strings.HasPrefix(dur, Asterisk) {
		err = fmt.Errorf("interval %q does not start with %q", dur, Asterisk)
		return
	}
	var n int64
	if n, err = strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(dur, Asterisk)), 10, 64); err == nil {
		d = scrapeInterval * time.Duration(n)
	}
	return
}

func adjustIntervalToScrapeInterval(cluster string, query string) (q string, si time.Duration) {
	q = query
	if cluster == Empty {
		return
	}
	for _, ifn := range intervalFunctions {
		var scrapeInterval time.Duration
		q, scrapeInterval = adjustIntervalsForFunction(cluster, q, ifn)
		if scrapeInterval > 0 && (si == 0 || scrapeInterval < si) {
			si = scrapeInterval
		}
	}
	return
}

func adjustIntervalsForFunction(cluster string, query string, ifn intervalFunction) (q string, si time.Duration) {
	var b strings.Builder
	for i := 0; i < len(query); {
		bodyStart, bodyEnd, ok := intervalFunctionCall(query, i, ifn)
		if !ok {
			b.WriteByte(query[i])
			i++
			continue
		}

		b.WriteString(query[i:bodyStart])
		body := query[bodyStart:bodyEnd]
		if j, k := intervalInFunctionBody(body); j > -1 {
			mName := metricNameFromExpression(body[:j])
			if mName == Empty {
				mName = metricNameBeforeRange(body, j)
			}
			scrapeInterval := getScrapeInterval(cluster, mName)
			if interval, changed := adjustedInterval(ifn, scrapeInterval, body[j+1:k]); changed {
				b.WriteString(body[:j+1])
				b.WriteString(interval)
				b.WriteString(body[k:])
				if scrapeInterval > 0 && (si == 0 || scrapeInterval < si) {
					si = scrapeInterval
				}
			} else {
				b.WriteString(body)
			}
		} else {
			b.WriteString(body)
		}
		b.WriteByte(query[bodyEnd])
		i = bodyEnd + 1
	}
	return b.String(), si
}

func intervalFunctionCall(query string, i int, ifn intervalFunction) (bodyStart, bodyEnd int, ok bool) {
	if ifn == intervalFunction(OverTimeSuffix) {
		if query[i] != leftBracket[0] {
			return
		}
		nameStart := i
		for nameStart > 0 && isPromFunctionNameChar(query[nameStart-1]) {
			nameStart--
		}
		if !strings.HasSuffix(query[nameStart:i], OverTimeSuffix) {
			return
		}
	} else {
		fn := string(ifn) + leftBracket
		if !strings.HasPrefix(query[i:], fn) || (i > 0 && isPromFunctionNameChar(query[i-1])) {
			return
		}
		i += len(ifn)
	}

	bodyStart = i + 1
	bodyEnd = matchingRightParen(query, i)
	ok = bodyEnd > -1
	return
}

func intervalInFunctionBody(body string) (start, end int) {
	parenDepth := 0
	inString := false
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c == '\\' && inString {
			i++
			continue
		}
		if c == DoubleQuote[0] {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch c {
		case leftBracket[0]:
			parenDepth++
		case RightBracket[0]:
			parenDepth--
		case leftSquareBracket[0]:
			if parenDepth == 0 {
				if k := strings.Index(body[i:], rightSquareBracket); k > -1 {
					return i, i + k
				}
			}
		}
	}
	return -1, -1
}

func adjustedInterval(ifn intervalFunction, scrapeInterval time.Duration, interval string) (string, bool) {
	switch ifn {
	case intervalFunction(OverTimeSuffix):
		parts := strings.SplitN(interval, ":", 2)
		if len(parts) == 2 {
			if d, err := scrapeIntervalMultiplication(scrapeInterval, strings.TrimSpace(parts[1])); err == nil {
				return parts[0] + ":" + d.String(), true
			}
		}
	case irate, idelta:
		if d, err := scrapeIntervalMultiplication(scrapeInterval, interval); err == nil {
			return d.String(), true
		}
	default:
		if d, err := time.ParseDuration(interval); err == nil {
			return (d + scrapeInterval).String(), true
		}
		if d, err := scrapeIntervalMultiplication(scrapeInterval, interval); err == nil {
			return d.String(), true
		}
	}
	return interval, false
}

func matchingRightParen(query string, left int) int {
	depth := 0
	inString := false
	for i := left; i < len(query); i++ {
		c := query[i]
		if c == '\\' && inString {
			i++
			continue
		}
		if c == DoubleQuote[0] {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch c {
		case leftBracket[0]:
			depth++
		case RightBracket[0]:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func metricNameFromExpression(expr string) string {
	if j := firstRangeStart(expr); j > -1 {
		return metricNameBeforeRange(expr, j)
	}
	return Empty
}

func firstRangeStart(expr string) int {
	inString := false
	for i := 0; i < len(expr); i++ {
		c := expr[i]
		if c == '\\' && inString {
			i++
			continue
		}
		if c == DoubleQuote[0] {
			inString = !inString
			continue
		}
		if !inString && c == leftSquareBracket[0] {
			return i
		}
	}
	return -1
}

func metricNameBeforeRange(expr string, rangeStart int) string {
	s := strings.TrimSpace(expr[:rangeStart])
	if strings.HasSuffix(s, rightBrace) {
		inString := false
		selectorStart := -1
		for i := 0; i < len(s); i++ {
			c := s[i]
			if c == '\\' && inString {
				i++
				continue
			}
			if c == DoubleQuote[0] {
				inString = !inString
				continue
			}
			if !inString && c == leftBrace[0] {
				selectorStart = i
			}
		}
		if selectorStart > -1 {
			s = s[:selectorStart]
		}
	}
	if i := strings.LastIndexAny(s, " \t(,+-*/%^:"); i > -1 {
		s = s[i+1:]
	}
	if i := strings.Index(s, leftBrace); i > -1 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func isPromFunctionNameChar(c byte) bool {
	return c == '_' || c == ':' || c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
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

func RegisterClusterQueryExclusion(name string, ce ClusterQueryExclusion) {
	clusterQueryExclusions[name] = ce
}

func UnregisterClusterQueryExclusion(name string) {
	delete(clusterQueryExclusions, name)
}

var clusterQueryExclusions = make(map[string]ClusterQueryExclusion)

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
	for mName, f := range m {
		mr := &metricResolver{metricName: mName, f: f}
		query := aggOverTimeQuery(mName+Braces, Present, Interval)
		query = fmt.Sprintf(`max(%s)`, query)
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
