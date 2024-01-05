package common

import "github.com/prometheus/common/model"

const (
	UnknownValue = -1
	mib          = 1024 * 1024
	milli        = 1000
)

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
