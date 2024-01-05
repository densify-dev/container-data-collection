package common

import (
	"context"
	"fmt"
	cconf "github.com/densify-dev/container-config/config"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/sigv4"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// CollectMetric is used to query Prometheus to get data for specific query and return the results to be processed
func CollectMetric(callDepth int, query string, promRange v1.Range) (crm ClusterResultMap, n int, err error) {
	cle := getClusterLabelsEmbedder(query)
	var qry string
	if qry, err = cle.embedClusterLabels(query); err != nil {
		return
	}
	for _, qlf := range labelFilters {
		queries := qlf.adjustQuery(qry)
		for cluster, q := range queries {
			if cluster == Empty {
				LogAll(callDepth+1, Debug, queryLogFormat, q)
			} else {
				LogCluster(callDepth+1, Debug, clusterQueryLogFormat, cluster, true, cluster, q)
			}
			var pa v1.API
			if pa, err = promApi(cluster); err != nil {
				return
			}
			ctx, cancel := context.WithCancel(context.Background())
			_ = time.AfterFunc(2*time.Minute, func() { cancel() })
			var value model.Value
			var e error
			value, _, e = pa.QueryRange(ctx, q, promRange)
			m := qlf.filterValue(cluster, q, value, e)
			if crm, err = Merge(crm, m, Fail); err != nil {
				break
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

func GetVersion() (version string, err error) {
	var pa v1.API
	ctx, cancel := context.WithCancel(context.Background())
	_ = time.AfterFunc(1*time.Minute, func() { cancel() })
	if pa, err = promApi(Empty); err != nil {
		return
	}
	var bir v1.BuildinfoResult
	if bir, err = pa.Buildinfo(ctx); err == nil {
		version = bir.Version
	}
	return
}

const (
	promClient  = "prometheus-client"
	labelPrefix = Label + Underscore
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
func TimeRange() (promRange v1.Range) {
	var d time.Duration
	return TimeRangeForInterval(d)
}

func TimeRangeForInterval(historyInterval time.Duration) (promRange v1.Range) {
	var start, end time.Time
	// for workload metrics the historyInterval will be set depending on how far back in history we are querying currently
	// note it will be 0 for all queries that are not workload related.
	intervalSize := time.Duration(Params.Collection.IntervalSize)
	switch Params.Collection.Interval {
	case Days:
		start = CurrentTime.Add(time.Hour * -24 * intervalSize).Add(time.Hour * -24 * intervalSize * historyInterval)
		end = CurrentTime.Add(time.Hour * -24 * intervalSize * historyInterval)
	case Hours:
		start = CurrentTime.Add(time.Hour * -1 * intervalSize).Add(time.Hour * -1 * intervalSize * historyInterval)
		end = CurrentTime.Add(time.Hour * -1 * intervalSize * historyInterval)
	default:
		start = CurrentTime.Add(time.Minute * -1 * intervalSize).Add(time.Minute * -1 * intervalSize * historyInterval)
		end = CurrentTime.Add(time.Minute * -1 * intervalSize * historyInterval)
	}
	return v1.Range{Start: start, End: end, Step: time.Minute * time.Duration(Params.Collection.SampleRate)}
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
