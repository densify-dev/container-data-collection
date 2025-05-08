package common

import (
	"github.com/prometheus/common/model"
	"strconv"
)

const (
	UnknownValue      = -1
	UnknownValueFloat = -1.0
	mib               = 1024 * 1024
	milli             = 1000
)

type ConvFunc[N Number] func(N) N

func MiB[N Number](n N) N {
	return n / N(mib)
}

func MCores[N Number](n N) N {
	return n * N(milli)
}

func IntMiB[N Number](n N) int {
	return int(MiB(n))
}

func IntMCores[N Number](n N) int {
	return int(MCores(n))
}

func LastValue(ss *model.SampleStream) float64 {
	return float64(LastSampleValue(ss))
}

func LastSampleValue(ss *model.SampleStream) (value model.SampleValue) {
	if len(ss.Values) > 0 {
		value = ss.Values[len(ss.Values)-1].Value
	}
	return
}

func GetValue(ss *model.SampleStream, name string) (s string) {
	s, _ = GetLabelValue(ss, name)
	return
}

func GetLabelValue(ss *model.SampleStream, name string) (s string, b bool) {
	var lv model.LabelValue
	lv, b = ss.Metric[model.LabelName(name)]
	s = string(lv)
	return
}

func GetLabelsValues(ss *model.SampleStream, names []string) (m map[string]string, b bool) {
	l := len(names)
	m = make(map[string]string, l)
	for _, name := range names {
		if value, f := GetLabelValue(ss, name); f {
			m[name] = value
		}
	}
	b = len(m) == l
	return
}

func SetMetricLabelValue[V any](cluster string, target *V, ss *model.SampleStream, key string) {
	if value, f := GetLabelValue(ss, key); f {
		setValue(cluster, target, key, value)
	}
}

func SetLabelValue[V any](cluster string, target *V, labels map[string]string, key string) {
	if value, f := labels[key]; f {
		setValue(cluster, target, key, value)
	}
}

func setValue[V any](cluster string, target *V, key, value string) {
	switch t := any(target).(type) {
	case *string:
		*t = value
	case *int:
		if v, err := strconv.Atoi(value); err == nil {
			*t = v
		}
	case *int64:
		if v, err := strconv.ParseInt(value, 10, 64); err == nil {
			*t = v
		}
	case *uint64:
		if v, err := strconv.ParseUint(value, 10, 64); err == nil {
			*t = v
		}
	case *bool:
		if v, err := strconv.ParseBool(value); err == nil {
			*t = v
		}
	default:
		LogCluster(1, Error, ClusterFormat+" unknown type %T for key %s and value %s in labels", cluster, true, cluster, t, key, value)
	}
}
