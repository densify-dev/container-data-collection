package common

import (
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type ClusterMatrixFunc func(string, model.Matrix)

const (
	QueryFormat        = "metric=%s query=%s"
	ClusterQueryFormat = ClusterFormat + Space + QueryFormat
)

func CollectAndProcessMetric(query string, promRange v1.Range, matrixFunc ClusterMatrixFunc) (n int, err error) {
	n, err = CollectAndProcessMetricErrorLogLevel(2, query, promRange, matrixFunc, Warn)
	return
}

func CollectAndProcessMetricErrorLogLevel(callDepth int, query string, promRange v1.Range, matrixFunc ClusterMatrixFunc, level LogLevel) (n int, err error) {
	callDepth++
	var resultMap ClusterResultMap
	resultMap, n, err = CollectMetric(callDepth, query, promRange)
	ProcessResultsErrorLogLevel(callDepth, resultMap, err, query, matrixFunc, level)
	return
}

func ProcessResults(callDepth int, resultMap ClusterResultMap, err error, query string, matrixFunc ClusterMatrixFunc) {
	ProcessResultsErrorLogLevel(callDepth+1, resultMap, err, query, matrixFunc, Warn)
}

func ProcessResultsErrorLogLevel(callDepth int, resultMap ClusterResultMap, err error, query string, matrixFunc ClusterMatrixFunc, level LogLevel) {
	if err != nil {
		LogErrorWithLevel(callDepth+1, level, err, QueryFormat, query)
	} else {
		for cluster, result := range resultMap {
			if result.Error == nil {
				if matrixFunc != nil {
					matrixFunc(cluster, result.Matrix)
				}
			} else {
				LogErrorWithLevel(callDepth+1, level, result.Error, ClusterQueryFormat, cluster, Empty, result.Query)
			}
		}
	}
}
