package common

import (
	"fmt"
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
	GetWorkload(2, wmh.fileName, wmh.metricName, query, metricField, entityKind)
}

func (wmh *WorkloadMetricHolder) GetWorkloadQueryVariants(callDepth int, qps map[string]*QueryProcessor, entityKind string) {
	GetWorkloadQueryVariantsFieldConversion(callDepth+1, wmh.fileName, wmh.metricName, qps, entityKind)
}

// common WorkloadMetricHolder structs
var (
	CpuUtilization           = NewWorkloadMetricHolder(Cpu, Utilization)
	MemoryBytes              = NewWorkloadMetricHolder(Memory, Bytes).OverrideFileName(Memory, Raw, Bytes)
	MemoryActualWorkload     = NewWorkloadMetricHolder(Memory, Actual, Workload)
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
	PodsLimits               = NewWorkloadMetricHolder(Pods, Limits).OverrideFileName(Pods)
	CpuReservationPercent    = NewWorkloadMetricHolder(Cpu, Reservation, Percent)
	MemoryReservationPercent = NewWorkloadMetricHolder(Memory, Reservation, Percent)
	PodCount                 = NewWorkloadMetricHolder(Pod, Count)
	OomKillEvents            = NewWorkloadMetricHolder(Oom, Kill, Events)
	CpuThrottlingEvents      = NewWorkloadMetricHolder(Cpu, Throttling, Events)
)

var conditionalQueries = map[bool][]string{
	true: {
		`avg(sum(kube_pod_container_resource_requests{resource="cpu"}) by (node)%s)`,
		`avg(sum(kube_pod_container_resource_requests{resource="cpu"}) by (node) / sum(kube_node_status_capacity{resource="cpu"}) by (node)%s) * 100`,
		`avg(sum(kube_pod_container_resource_requests{resource="memory"}/1024/1024) by (node)%s)`,
		`avg(sum(kube_pod_container_resource_requests{resource="memory"}/1024/1024) by (node) / sum(kube_node_status_capacity{resource="memory"}/1024/1024) by (node)%s) * 100`,
	},
	false: {
		`avg(sum(kube_pod_container_resource_requests_cpu_cores{}) by (node)%s)`,
		`avg(sum(kube_pod_container_resource_requests_cpu_cores{}) by (node) / sum(kube_node_status_capacity_cpu_cores{}) by (node)%s) * 100`,
		`avg(sum(kube_pod_container_resource_requests_memory_bytes{}/1024/1024) by (node)%s)`,
		`avg(sum(kube_pod_container_resource_requests_memory_bytes{}/1024/1024) by (node) / sum(kube_node_status_capacity_memory_bytes{}/1024/1024) by (node)%s) * 100`,
	},
}

var conditionalMetricHolders = []*WorkloadMetricHolder{
	CpuRequests,
	CpuReservationPercent,
	MemoryRequests,
	MemoryReservationPercent,
}

func GetConditionalMetricsWorkload(indicators map[string]int, indicator string, querySubToMetricFields map[string][]model.LabelName, entityKind string) {
	for _, f := range FoundIndicatorCounter(indicators, indicator) {
		for i, q := range conditionalQueries[f] {
			// substitute querySub in query and recreate queryToMetricFields map
			qps := make(map[string]*QueryProcessor, len(querySubToMetricFields))
			for querySub, metricFields := range querySubToMetricFields {
				query := fmt.Sprintf(q, querySub)
				qps[query] = &QueryProcessor{MetricFields: metricFields}
			}
			cmh := conditionalMetricHolders[i]
			GetWorkloadQueryVariants(2, cmh.fileName, cmh.metricName, qps, entityKind)
		}
	}
}

type QueryProcessor struct {
	MetricFields []model.LabelName
	FF           FieldsFunc
}

// GetWorkload used to query for the workload data and then calls write workload
func GetWorkload(callDepth int, fileName, metricName, query string, metricFields []model.LabelName, entityKind string) {
	qps := map[string]*QueryProcessor{query: {MetricFields: metricFields}}
	GetWorkloadQueryVariants(callDepth+1, fileName, metricName, qps, entityKind)
}

func GetWorkloadQueryVariants(callDepth int, fileName, metricName string, queryProcessors map[string]*QueryProcessor, entityKind string) {
	GetWorkloadQueryVariantsFieldConversion(callDepth+1, fileName, metricName, queryProcessors, entityKind)
}

func GetWorkloadQueryVariantsFieldConversion(callDepth int, fileName, metricName string, queryProcessors map[string]*QueryProcessor, entityKind string) {
	csvHeaderFormat, f := GetCsvHeaderFormat(entityKind)
	if !f {
		LogError(fmt.Errorf("no CSV header format found"), EntityFormat)
		return
	}
	clusterFiles := make(map[string]*os.File)
	//If the History parameter is set to anything but default 1 then will loop through the calls starting with the current day\hour\minute interval and work backwards.
	//This is done as the farther you go back in time the slower prometheus querying becomes and we have seen cases where will not run from timeouts on Prometheus.
	//As a result if we do hit an issue with timing out on Prometheus side we still can send the current data and data going back to that point vs losing it all.
	for historyInterval := 0; historyInterval < Params.Collection.HistoryInt; historyInterval++ {
		range5Min := TimeRangeForInterval(time.Duration(historyInterval))
		for query, qp := range queryProcessors {
			if crm, _, err := CollectMetric(callDepth+1, query, range5Min); err != nil {
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
						fp := &FieldProvider{Cluster: cluster, MetricFields: qp.MetricFields, ConvF: qp.FF}
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
			if err := WriteValues(file, clusterName, f, ss.Values); err != nil {
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

func WriteValues(file io.Writer, clusterName, fields string, values []model.SamplePair) error {
	for _, value := range values {
		var val model.SampleValue
		if fval := float64(value.Value); !math.IsNaN(fval) && !math.IsInf(fval, 0) {
			val = value.Value
		}
		var err error
		if _, err = fmt.Fprintf(file, "%s,", clusterName); err != nil {
			return err
		}
		if fields != Empty {
			if _, err = fmt.Fprintf(file, "%s,", ReplaceSemiColons(fields)); err != nil {
				return err
			}
		}
		if _, err = fmt.Fprintf(file, "%s,%f\n", FormatTime(value.Timestamp), val); err != nil {
			return err
		}
	}
	return nil
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
			hf, _ := GetCsvHeaderFormat(ek)
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
