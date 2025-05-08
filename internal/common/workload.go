package common

import (
	"fmt"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"io"
	"math"
	"os"
	"time"
)

type NameType uint

const (
	MetricName NameType = iota
	FileName
)

type WorkloadMetricHolder struct {
	fileName, metricName string
}

func NewWorkloadMetricHolder(nameElements ...string) *WorkloadMetricHolder {
	name := JoinSpace(nameElements...)
	return &WorkloadMetricHolder{
		fileName:   snakeCase(name),
		metricName: camelCase(name),
	}
}

func (wmh *WorkloadMetricHolder) OverrideFileName(nameElements ...string) *WorkloadMetricHolder {
	wmh.fileName = SnakeCase(nameElements...)
	return wmh
}

func (wmh *WorkloadMetricHolder) GetMetricName() string {
	return wmh.metricName
}

func (wmh *WorkloadMetricHolder) GetFileName() string {
	return wmh.fileName
}

func (wmh *WorkloadMetricHolder) GetName(nt NameType, singular bool) (name string) {
	switch nt {
	case MetricName:
		name = wmh.GetMetricName()
	case FileName:
		name = wmh.GetFileName()
	}
	if singular {
		name = Singular(name)
	}
	return
}

func (wmh *WorkloadMetricHolder) GetWorkload(query string, metricField []model.LabelName, entityKind string) {
	wmh.GetWorkloadFieldsFunc(query, metricField, nil, entityKind)
}

// GetWorkloadFieldsFunc - call this function directly ONLY if you need to provide the FieldsFunc;
// otherwise use GetWorkload
func (wmh *WorkloadMetricHolder) GetWorkloadFieldsFunc(query string, metricField []model.LabelName, ff FieldsFunc, entityKind string) {
	callDepth := 2
	if ff == nil {
		callDepth++
	}
	GetWorkload(callDepth, wmh.fileName, wmh.metricName, query, metricField, ff, entityKind, Metric, nil)
}

func (wmh *WorkloadMetricHolder) GetWorkloadQueryVariants(callDepth int, qps map[string]*QueryProcessor, entityKind string) {
	GetWorkloadQueryVariantsFieldConversion(callDepth+1, wmh.fileName, wmh.metricName, qps, entityKind, Metric, nil)
}

// common WorkloadMetricHolder structs
var (
	CpuUtilization           = NewWorkloadMetricHolder(Cpu, Utilization)
	MemoryBytes              = NewWorkloadMetricHolder(Memory, Bytes).OverrideFileName(Memory, Raw, Bytes)
	MemoryActualWorkload     = NewWorkloadMetricHolder(Memory, Actual, Workload)
	MemoryWs                 = NewWorkloadMetricHolder(Memory, WorkingSet)
	MemoryUtilization        = NewWorkloadMetricHolder(Memory, Utilization)
	MemoryActualUtilization  = NewWorkloadMetricHolder(Memory, Actual, Utilization)
	MemoryWsUtilization      = NewWorkloadMetricHolder(Memory, WorkingSet, Utilization)
	GpuUtilizationAvg        = NewWorkloadMetricHolder(Gpu, Utilization, Avg)
	GpuUtilizationMax        = NewWorkloadMetricHolder(Gpu, Utilization, Max)
	GpuUtilizationGpusAvg    = NewWorkloadMetricHolder(Gpu, Utilization, Gpus, Avg)
	GpuUtilizationGpusMax    = NewWorkloadMetricHolder(Gpu, Utilization, Gpus, Max)
	GpuMemUtilizationAvg     = NewWorkloadMetricHolder(Gpu, Mem, Utilization, Avg)
	GpuMemUtilizationMax     = NewWorkloadMetricHolder(Gpu, Mem, Utilization, Max)
	GpuMemUsedAvg            = NewWorkloadMetricHolder(Gpu, Mem, Used, Avg)
	GpuMemUsedMax            = NewWorkloadMetricHolder(Gpu, Mem, Used, Max)
	GpuPowerUsageAvg         = NewWorkloadMetricHolder(Gpu, Power, Usage, Avg)
	GpuPowerUsageMax         = NewWorkloadMetricHolder(Gpu, Power, Usage, Max)
	DiskReadBytes            = NewWorkloadMetricHolder(Disk, Read, Bytes)
	DiskWriteBytes           = NewWorkloadMetricHolder(Disk, Write, Bytes)
	DiskTotalBytes           = NewWorkloadMetricHolder(Disk, Total, Bytes)
	DiskReadOps              = NewWorkloadMetricHolder(Disk, Read, Ops)
	DiskWriteOps             = NewWorkloadMetricHolder(Disk, Write, Ops)
	DiskTotalOps             = NewWorkloadMetricHolder(Disk, Total, Ops)
	NetReceivedBytes         = NewWorkloadMetricHolder(Net, Received, Bytes)
	NetSentBytes             = NewWorkloadMetricHolder(Net, Sent, Bytes)
	NetTotalBytes            = NewWorkloadMetricHolder(Net, Total, Bytes)
	NetReceivedPackets       = NewWorkloadMetricHolder(Net, Received, Packets)
	NetSentPackets           = NewWorkloadMetricHolder(Net, Sent, Packets)
	NetTotalPackets          = NewWorkloadMetricHolder(Net, Total, Packets)
	CurrentSize              = NewWorkloadMetricHolder(Current, Size)
	CpuLimits                = NewWorkloadMetricHolder(Cpu, Limits)
	CpuRequests              = NewWorkloadMetricHolder(Cpu, Requests)
	MemoryLimits             = NewWorkloadMetricHolder(Memory, Limits)
	MemoryRequests           = NewWorkloadMetricHolder(Memory, Requests)
	MemLimits                = NewWorkloadMetricHolder(Mem, Limits)
	MemRequests              = NewWorkloadMetricHolder(Mem, Requests)
	GpuLimits                = NewWorkloadMetricHolder(Gpu, Limits)
	GpuRequests              = NewWorkloadMetricHolder(Gpu, Requests)
	PodsLimits               = NewWorkloadMetricHolder(Pods, Limits).OverrideFileName(Pods)
	CpuReservationPercent    = NewWorkloadMetricHolder(Cpu, Reservation, Percent)
	MemoryReservationPercent = NewWorkloadMetricHolder(Memory, Reservation, Percent)
	PodCount                 = NewWorkloadMetricHolder(Pod, Count)
	OomKillEvents            = NewWorkloadMetricHolder(Oom, Kill, Events)
	CpuThrottlingEvents      = NewWorkloadMetricHolder(Cpu, Throttling, Events)
)

const (
	FilterTerminatedContainersClause = ` unless on (namespace,pod) max(kube_pod_status_phase{phase!="Running"}) by (namespace,pod) == 1 or on (namespace,pod,container) max(kube_pod_container_status_terminated{} or kube_pod_container_status_terminated_reason{}) by (namespace,pod,container) == 1`
)

func FilterTerminatedContainers(prefix, suffix string) string {
	return prefix + FilterTerminatedContainersClause + suffix
}

var conditionalQueries = map[bool][]string{
	true: {
		FilterTerminatedContainers(`sum(sum(kube_pod_container_resource_requests{resource="cpu"}`, `) by (node)%s)`),
		FilterTerminatedContainers(`avg(sum(kube_pod_container_resource_requests{resource="cpu"}`, `) by (node) / sum(kube_node_status_allocatable{resource="cpu"}) by (node)%s) * 100`),
		FilterTerminatedContainers(`sum(sum(kube_pod_container_resource_requests{resource="memory"}/1024/1024`, `) by (node)%s)`),
		FilterTerminatedContainers(`avg(sum(kube_pod_container_resource_requests{resource="memory"}/1024/1024`, `) by (node) / sum(kube_node_status_allocatable{resource="memory"}/1024/1024) by (node)%s) * 100`),
	},
	false: {
		FilterTerminatedContainers(`sum(sum(kube_pod_container_resource_requests_cpu_cores{}`, `) by (node)%s)`),
		FilterTerminatedContainers(`avg(sum(kube_pod_container_resource_requests_cpu_cores{}`, `) by (node) / sum(kube_node_status_allocatable_cpu_cores{}) by (node)%s) * 100`),
		FilterTerminatedContainers(`sum(sum(kube_pod_container_resource_requests_memory_bytes{}/1024/1024`, `) by (node)%s)`),
		FilterTerminatedContainers(`avg(sum(kube_pod_container_resource_requests_memory_bytes{}/1024/1024`, `) by (node) / sum(kube_node_status_allocatable_memory_bytes{}/1024/1024) by (node)%s) * 100`),
	},
}

var conditionalMetricHolders = []*WorkloadMetricHolder{
	CpuRequests,
	CpuReservationPercent,
	MemoryRequests,
	MemoryReservationPercent,
}

func GetConditionalMetricsWorkload(indicators map[string]int, indicator string, querySubToMetricFields map[string][]model.LabelName, entityKind string, subject string) {
	for _, f := range FoundIndicatorCounter(indicators, indicator) {
		for i, q := range conditionalQueries[f] {
			// substitute querySub in query and recreate queryToMetricFields map
			qps := make(map[string]*QueryProcessor, len(querySubToMetricFields))
			for querySub, metricFields := range querySubToMetricFields {
				query := fmt.Sprintf(q, querySub)
				qps[query] = &QueryProcessor{MetricFields: metricFields}
			}
			cmh := conditionalMetricHolders[i]
			GetWorkloadQueryVariants(2, cmh.fileName, cmh.metricName, qps, entityKind, subject, nil)
		}
	}
}

type QueryProcessor struct {
	MetricFields []model.LabelName
	FF           FieldsFunc
}

// GetWorkload used to query for the workload data and then calls write workload
func GetWorkload(callDepth int, fileName, metricName, query string, metricFields []model.LabelName, ff FieldsFunc, entityKind string, subject string, tvp QueryProvider) {
	qps := map[string]*QueryProcessor{query: {MetricFields: metricFields, FF: ff}}
	GetWorkloadQueryVariants(callDepth+1, fileName, metricName, qps, entityKind, subject, tvp)
}

func GetWorkloadQueryVariants(callDepth int, fileName, metricName string, queryProcessors map[string]*QueryProcessor, entityKind string, subject string, tvp QueryProvider) {
	GetWorkloadQueryVariantsFieldConversion(callDepth+1, fileName, metricName, queryProcessors, entityKind, subject, tvp)
}

func GetWorkloadQueryVariantsFieldConversion(callDepth int, fileName, metricName string, queryProcessors map[string]*QueryProcessor, entityKind string, subject string, qp QueryProvider) {
	prov := queryProviderOrDefault(qp)
	csvHeaderFormat, f := GetCsvHeaderFormat(entityKind, subject)
	if !f {
		LogError(fmt.Errorf("no CSV header format found"), EntityFormat)
		return
	}
	clusterFiles := make(map[string]*os.File)
	//If the History parameter is set to anything but default 1 then will loop through the calls starting with the current day\hour\minute interval and work backwards.
	//This is done as the farther you go back in time the slower prometheus querying becomes and we have seen cases where will not run from timeouts on Prometheus.
	//As a result if we do hit an issue with timing out on Prometheus side we still can send the current data and data going back to that point vs losing it all.
	for historyInterval := 0; historyInterval < Params.Collection.HistoryInt; historyInterval++ {
		rng := prov.CalculateRange(historyInterval)
		for query, qp := range queryProcessors {
			if crm, _, err := CollectMetric(callDepth+1, query, rng); err != nil {
				LogErrorWithLevel(1, Warn, err, QueryFormat, metricName, query)
			} else {
				for cluster, result := range crm {
					if result == nil || result.Matrix.Len() == 0 {
						continue
					}
					file, initialized := clusterFiles[cluster]
					if !initialized {
						file = InitWorkloadFile(cluster, fileName, entityKind, csvHeaderFormat, metricName)
						clusterFiles[cluster] = file
					}
					if file != nil {
						fp := &FieldProvider{Cluster: cluster, MetricFields: qp.MetricFields, ConvF: qp.FF, QProv: prov}
						if err = writeWorkload(file, cluster, result.Matrix, fp); err != nil {
							LogError(err, ClusterFileFormat, cluster, fileName)
						}
					}
				}
			}
		}
	}
	// close the workload files
	for cluster, file := range clusterFiles {
		if file != nil {
			if err := file.Close(); err != nil {
				LogError(err, ClusterFileFormat, cluster, fileName)
			}
		}
	}
}

func InitWorkloadFile(cluster, fileName, entityKind, csvHeaderFormat, metricName string) *os.File {
	var err error
	if _, err = os.Stat(fileName); err == nil {
		err = fmt.Errorf("%s %v", fileName, os.ErrExist)
		LogError(err, DefaultLogFormat, cluster, entityKind)
		return nil
	}
	var workloadWrite *os.File
	if workloadWrite, err = os.Create(GetFileName(cluster, entityKind, fileName)); err != nil {
		LogError(err, DefaultLogFormat, cluster, entityKind)
		return nil
	}
	_, err = fmt.Fprintf(workloadWrite, csvHeaderFormat, metricName)
	if err != nil {
		LogError(err, DefaultLogFormat, cluster, entityKind)
		_ = workloadWrite.Close()
		return nil
	}
	return workloadWrite
}

func writeWorkload(file io.Writer, clusterName string, result model.Matrix, fp *FieldProvider) error {
	for _, ss := range result {
		if f, ok := fp.Fields(ss.Metric); ok {
			if err := WriteValues(file, clusterName, f, ss.Values, fp.QProv); err != nil {
				return err
			}
		}
	}
	return nil
}

type FieldsFunc func(string, []string) ([]string, bool)

type FieldProvider struct {
	Cluster      string
	MetricFields []model.LabelName
	ConvF        FieldsFunc
	QProv        QueryProvider
}

func (fp *FieldProvider) Fields(metric model.Metric) (string, bool) {
	var f string
	var fields []string
	ok := true
	for _, mf := range fp.MetricFields {
		var field model.LabelValue
		if field, ok = metric[mf]; ok {
			fields = append(fields, string(field))
		} else {
			break
		}
	}
	if ok && fp.ConvF != nil {
		fields, ok = fp.ConvF(fp.Cluster, fields)
	}
	if ok {
		f = JoinComma(fields...)
	}
	return f, ok
}

func WriteValues(file io.Writer, clusterName, fields string, values []model.SamplePair, qp QueryProvider) error {
	for _, value := range values {
		if !IsValidValue(&value) {
			continue
		}
		prov := queryProviderOrDefault(qp)
		if tv := prov.TimeAndValues(&value); tv != nil {
			for i := 0; i < tv.Count; i++ {
				var err error
				if _, err = fmt.Fprintf(file, "%s,", clusterName); err != nil {
					return err
				}
				if fields != Empty {
					if _, err = fmt.Fprintf(file, "%s,", ReplaceSemiColons(fields)); err != nil {
						return err
					}
				}
				if _, err = fmt.Fprintf(file, "%s,%s\n", FormatTime(tv.Time), tv.Values); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

type TimeAndValues struct {
	Time   model.Time
	Values string
	Count  int
}

type QueryProvider interface {
	TimeAndValues(value *model.SamplePair) *TimeAndValues
	CalculateRange(historyInterval int) *v1.Range
}

func IsValidValue(value *model.SamplePair) bool {
	f := float64(value.Value)
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

type MetricTimeAndValuesProvider struct {
}

func (mtvp *MetricTimeAndValuesProvider) TimeAndValues(value *model.SamplePair) *TimeAndValues {
	return &TimeAndValues{
		Time:   value.Timestamp,
		Values: fmt.Sprintf("%f", value.Value),
		Count:  1,
	}
}

func (mtvp *MetricTimeAndValuesProvider) CalculateRange(historyInterval int) *v1.Range {
	return TimeRangeForInterval(time.Duration(historyInterval))
}

var metricTimeAndValuesProvider = &MetricTimeAndValuesProvider{}

func queryProviderOrDefault(qp QueryProvider) QueryProvider {
	if qp == nil {
		return metricTimeAndValuesProvider
	} else {
		return qp
	}
}

type ClusterWorkloadWriters map[string]*os.File
type WorkloadWriters map[string]ClusterWorkloadWriters

func NewWorkloadWriters() WorkloadWriters {
	return make(WorkloadWriters)
}

func (wws WorkloadWriters) AddMetricWorkloadWriters(wmhs ...*WorkloadMetricHolder) {
	for _, wmh := range wmhs {
		metric := wmh.GetName(MetricName, true)
		wws[metric] = make(ClusterWorkloadWriters)
	}
}

func (wws WorkloadWriters) CloseAndClearWorkloadWriters(entityKind string) {
	for _, cws := range wws {
		for cluster, file := range cws {
			if err := file.Close(); err != nil {
				LogError(err, DefaultLogFormat, cluster, entityKind)
			}
		}
		clear(cws)
	}
	clear(wws)
}

type WorkloadProducer interface {
	GetCluster() string
	GetEntityKind() string
	GetRowPrefixes() []string
	ShouldWrite(metric string) bool
}

func WriteWorkload(wp WorkloadProducer, wws WorkloadWriters, wmh *WorkloadMetricHolder, ss *model.SampleStream, f ConvFunc[float64]) {
	metric := wmh.GetName(MetricName, true)
	if !wp.ShouldWrite(metric) {
		return
	}
	cluster := wp.GetCluster()
	ek := wp.GetEntityKind()
	var file *os.File
	var err error
	if file = wws[metric][cluster]; file == nil {
		if file, err = os.Create(GetFileName(cluster, ek, wmh.GetName(FileName, true))); err == nil {
			hf, _ := GetCsvHeaderFormat(ek, Metric)
			if _, err = fmt.Fprintf(file, hf, metric); err == nil {
				wws[metric][cluster] = file
			} else {
				LogError(err, DefaultLogFormat, cluster, ek)
				_ = file.Close()
			}
		} else {
			LogError(err, DefaultLogFormat, cluster, ek)
		}
	}
	if err == nil && file != nil {
		vals := make([]float64, len(ss.Values))
		times := make([]string, len(ss.Values))
		for i, value := range ss.Values {
			val := float64(value.Value)
			if f != nil {
				val = f(val)
			}
			vals[i] = val
			times[i] = FormatTime(value.Timestamp)
		}
	outer:
		for _, rowPrefix := range wp.GetRowPrefixes() {
			for i, t := range times {
				if _, err = fmt.Fprintf(file, "%s,%s,%f\n", rowPrefix, t, vals[i]); err != nil {
					LogError(err, DefaultLogFormat, cluster, ek)
					break outer
				}
			}
		}
	}
}
