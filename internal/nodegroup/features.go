package nodegroup

import (
	"fmt"

	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/densify-dev/container-data-collection/internal/node"
	"github.com/prometheus/common/model"
)

type featureType string

const (
	labelType     featureType = "label"
	openshiftType featureType = "openshift"
	roleType      featureType = "role"
)

type clusterFeature interface {
	Type() featureType
	NodeAndGroupCoreQueryFmt() string
	LabelNames() []model.LabelName
	AdjustNodeGroupName(name string) string
}

var clusterFeatures = make(map[string]clusterFeature)

func AdjustNodeGroupName(cluster, name string) (s string) {
	if cf, f := clusterFeatures[cluster]; f {
		s = cf.AdjustNodeGroupName(name)
	} else {
		s = name
	}
	return
}

func QuerySuffixFmt(cf clusterFeature, queryTarget string, closeBracket bool, extraArgs ...string) (s string) {
	s = " * on (node) group_"
	switch queryTarget {
	case common.Metric:
		s += "right "
	case common.ConfigSt:
		s += "left (%v) "
	}
	s += cf.NodeAndGroupCoreQueryFmt()
	s += ") by (%v"
	for _ = range len(extraArgs) {
		s += ", %s"
	}
	if closeBracket {
		s += common.RightBracket
	}
	return
}

func ByPodIpMetricSuffixFmt(cf clusterFeature) string {
	return node.ByPodIpSuffix + QuerySuffixFmt(cf, common.Metric, true)
}

func QueryWrappersMap(cf clusterFeature) map[string]*node.QueryWrapper {
	suffix := QuerySuffixFmt(cf, common.Metric, true)
	podIpSuffix := ByPodIpMetricSuffixFmt(cf)
	return map[string]*node.QueryWrapper{
		node.HasInstanceLabelPodIp: {
			Query: &common.WorkloadQueryWrapper{
				Prefix: "sum(max(label_replace(",
				Suffix: podIpSuffix,
			},
			SumQuery: &common.WorkloadQueryWrapper{
				Prefix: "sum(sum(label_replace(",
				Suffix: podIpSuffix,
			},
			MetricField: []model.LabelName{common.Node},
		},
		node.HasNodeLabel: {
			Query: &common.WorkloadQueryWrapper{
				Prefix: "sum(",
				Suffix: suffix,
			},
			SumQuery: &common.WorkloadQueryWrapper{
				Prefix: "sum(sum(",
				Suffix: `) by (node)` + suffix,
			},
			MetricField: []model.LabelName{common.Node},
		},
		node.HasInstanceLabelOther: {
			Query: &common.WorkloadQueryWrapper{
				Prefix: "sum(label_replace(",
				Suffix: `, "node", "$1", "instance", "(.*)")` + suffix,
			},
			SumQuery: &common.WorkloadQueryWrapper{
				Prefix: "sum(label_replace(sum(",
				Suffix: `) by (instance), "node", "$1", "instance", "(.*)")` + suffix,
			},
			MetricField: []model.LabelName{common.Instance},
		},
	}
}

var countWqw = &common.WorkloadQueryWrapper{
	Prefix: "sum(",
	Suffix: fmt.Sprintf(") by (%s)", common.DefaultFmt),
}

func CountQueryFmt(cf clusterFeature) string {
	return countWqw.Wrap(cf.NodeAndGroupCoreQueryFmt())
}

func ensureFeature(cluster string, ft featureType) (cf clusterFeature, f bool) {
	if cf, f = clusterFeatures[cluster]; !f {
		switch ft {
		case labelType:
			cf = &labelFeature{}
		case openshiftType:
			cf = &openshiftFeature{
				detectedMachineSets:      make(map[string]bool),
				machineSetsSubstitutions: make(map[string]string),
			}
		case roleType:
			cf = &roleFeature{}
		}
		if cf != nil {
			clusterFeatures[cluster], f = cf, true
		}
	}
	return
}

func ensureLabelFeature(cluster string) (lf *labelFeature, ok bool) {
	var cf clusterFeature
	if cf, ok = ensureFeature(cluster, labelType); ok {
		if ok = cf.Type() == labelType; ok {
			lf, ok = cf.(*labelFeature)
		}
	}
	return
}

func ensureOpenShiftFeature(cluster string) (of *openshiftFeature, ok bool) {
	var cf clusterFeature
	if cf, ok = ensureFeature(cluster, openshiftType); ok {
		if ok = cf.Type() == openshiftType; ok {
			of, ok = cf.(*openshiftFeature)
		}
	}
	return
}

func ensureRoleFeature(cluster string) (rf *roleFeature, ok bool) {
	var cf clusterFeature
	if cf, ok = ensureFeature(cluster, roleType); ok {
		if ok = cf.Type() == roleType; ok {
			rf, ok = cf.(*roleFeature)
		}
	}
	return
}
