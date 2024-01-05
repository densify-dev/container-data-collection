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
