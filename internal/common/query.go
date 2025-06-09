package common

import (
	"fmt"
	"strings"
)

const (
	labelsPlaceholderBraces = leftBrace + labelsPlaceholder + rightBrace
	commaLabelsPlaceholder  = Comma + labelsPlaceholder
)

type clusterLabelsEmbedder interface {
	embedClusterLabels(query string) (string, error)
}

type queryType int

const (
	plain queryType = iota
	aggregator
	labeled
)

var cles = make(map[queryType]map[int]clusterLabelsEmbedder)

func createRange(n int) (r []int) {
	r = make([]int, n)
	for i := range r {
		r[i] = i
	}
	return
}

func getClusterLabelsEmbedder(query string) (cle clusterLabelsEmbedder) {
	var qt queryType
	var n int
	if k := strings.Count(query, rightBrace); k > 0 {
		qt, n = labeled, k
	} else if strings.Contains(query, rightBracket) {
		qt, n = aggregator, 1
	} else {
		qt = plain
	}
	return getLabelsEmbedder(qt, n)
}

func getLabelsEmbedder(qt queryType, n int) clusterLabelsEmbedder {
	var m map[int]clusterLabelsEmbedder
	var f bool
	if m, f = cles[qt]; !f {
		m = make(map[int]clusterLabelsEmbedder)
		cles[qt] = m
	}
	var cle clusterLabelsEmbedder
	if cle, f = m[n]; !f {
		switch qt {
		case plain:
			cle = &plainQuery{}
		case aggregator:
			cle = aggregatorQuery(createRange(n)...)
		case labeled:
			cle = queryWithLabels(createRange(n)...)
		}
		m[n] = cle
	}
	return cle
}

type plainQuery struct {
}

func (pq *plainQuery) embedClusterLabels(query string) (string, error) {
	return query + labelsPlaceholderBraces, nil
}

type complexQuery struct {
	rightBracketIndices []int
	rightBraceIndices   []int
}

func aggregatorQuery(indices ...int) clusterLabelsEmbedder {
	return getComplexQuery(indices, nil)
}

func queryWithLabels(indices ...int) clusterLabelsEmbedder {
	return getComplexQuery(nil, indices)
}

func getComplexQuery(rightBracketIndices, rightBraceIndices []int) clusterLabelsEmbedder {
	if len(rightBracketIndices) == 0 && len(rightBraceIndices) == 0 {
		panic("error: ComplexQuery called with no arguments")
	}
	return &complexQuery{
		rightBracketIndices: rightBracketIndices,
		rightBraceIndices:   rightBraceIndices,
	}
}

func (cq *complexQuery) embedClusterLabels(query string) (q string, err error) {
	/*
		if q, err = embedClusterLabels(query, rightBracket, cq.rightBracketIndices, labelsPlaceholderBraces); err == nil {
			q, err = embedClusterLabels(q, rightBrace, cq.rightBraceIndices, commaLabelsPlaceholder)
		}
	*/
	q, err = embedClusterLabels(query, rightBrace, cq.rightBraceIndices, commaLabelsPlaceholder)
	return
}

func embedClusterLabels(query string, search string, indices []int, embed string) (string, error) {
	if len(indices) == 0 {
		return query, nil
	}
	if search == Empty {
		return Empty, fmt.Errorf("embedClusterLabels: search string is Empty")
	}
	if !strings.Contains(query, search) {
		return Empty, fmt.Errorf("embedClusterLabels: query %s does not include search string %s", query, search)
	}
	s := strings.Split(query, search)
	n := len(s)
	for _, index := range indices {
		if index >= n {
			return Empty, fmt.Errorf("embedClusterLabels: requested index %d but only %d found for search string %s in query %s", index, n, search, query)
		}
		s[index] = s[index] + embed
	}
	return Join(search, s...), nil
}

type WorkloadQueryWrapper struct {
	Prefix, Suffix string
}

func (wqw *WorkloadQueryWrapper) Wrap(query string) string {
	return wqw.Prefix + query + wqw.Suffix
}

type WrapperGenerator func(string) string

func (wqw *WorkloadQueryWrapper) GenerateWrapper(wgPrefix, wgSuffix WrapperGenerator) (nwqw *WorkloadQueryWrapper) {
	if wqw != nil {
		nwqw = &WorkloadQueryWrapper{
			Prefix: generateOrOrigin(wqw.Prefix, wgPrefix),
			Suffix: generateOrOrigin(wqw.Suffix, wgSuffix),
		}
	}
	return
}

func generateOrOrigin(s string, wg WrapperGenerator) string {
	if wg == nil {
		return s
	} else {
		return wg(s)
	}
}

type LabelReplaceCondition int

const (
	HasValue LabelReplaceCondition = iota
	Always
)

const (
	hasValueStr = ".+"
	alwaysStr   = ".*"
)

func (lrc LabelReplaceCondition) String() (s string) {
	switch lrc {
	case HasValue:
		s = hasValueStr
	case Always:
		s = alwaysStr
	}
	return
}

func LabelReplace(query, dstLabel, srcLabel string, lrc LabelReplaceCondition) string {
	return fmt.Sprintf(`label_replace(%s, "%s", "$1", "%s", "(%s)")`, query, dstLabel, srcLabel, lrc.String())
}

func DcgmExporterLabelReplace(query string) string {
	return LabelReplace(query, Node, Hostname, Always)
}

func AggOverTimeQuery(q string, agg string) string {
	return fmt.Sprintf("%s_over_time(%s[%v:])", agg, q, Step)
}

func DcgmAggOverTimeQuery(q string, agg string) string {
	return AggOverTimeQuery(DcgmExporterLabelReplace(q), agg)
}

// SafeDcgmGpuUtilizationQuery is a query that checks if the GPU utilization is capped by 100%.
// Required as in some cases in the wild we've observed values like 100,000%, which mess up averaging and other calculations.
// Not clear if this was caused by a GPU which is pegged to 100% or an issue in DCGM exporter
var SafeDcgmGpuUtilizationQuery = fmt.Sprintf("(%s <= 100)", DcgmExporterLabelReplace("DCGM_FI_DEV_GPU_UTIL{}"))

func DcgmPercentQuerySuffix(metric string, onWhat ...string) string {
	what := strings.Join(onWhat, Comma)
	return fmt.Sprintf(` * on (%s) %s{%s="%s"} / 100`, what, metric, Resource, NvidiaGpuResource)
}
