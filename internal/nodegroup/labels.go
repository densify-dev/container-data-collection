package nodegroup

import (
	"cmp"
	"fmt"

	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/prometheus/common/model"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

type NodeLabelProviderType int

const (
	CustomLabel NodeLabelProviderType = iota
	ClusterManager
	NodeAutoscaler
	CloudServiceProvider
	ClusterProvisioningTool
	UnclassifiedProvider // must be last
)

var allowedLowerLabelsProviders = map[NodeLabelProviderType]map[NodeLabelProviderType]bool{
	NodeAutoscaler: {CloudServiceProvider: true},
}

// well-known labels
const (
	GardenerLabel  model.LabelName = "label_worker_gardener_cloud_pool"
	KarpenterLabel model.LabelName = "label_karpenter_sh_nodepool"
	GkeLabel       model.LabelName = "label_cloud_google_com_gke_nodepool"
	EksLabel       model.LabelName = "label_eks_amazonaws_com_nodegroup"
	AksLabel       model.LabelName = "label_agentpool"
	EksctlLabel    model.LabelName = "label_alpha_eksctl_io_nodegroup_name"
	KopsLabel      model.LabelName = "label_kops_k8s_io_instancegroup"
	PoolNameLabel  model.LabelName = "label_pool_name"
)

const (
	// DefaultNodeGroup cannot be "default" as some provisioners (e.g. Karpenter) may use this value
	DefaultNodeGroup model.LabelValue = "__default__"
	NodeInfoMetric                    = "kube_node_info"
	NodeLabelMetric                   = "kube_node_labels"
	NodeRoleMetric                    = "kube_node_role"
	NonExistingLabel model.LabelName  = "__non_existing_nodegroup_label__"
)

// nodeLabelToProviderType - no label's provider type should have a value of CustomLabel
var nodeLabelToProviderType = map[model.LabelName]NodeLabelProviderType{
	GardenerLabel:  ClusterManager,
	KarpenterLabel: NodeAutoscaler,
	GkeLabel:       CloudServiceProvider,
	EksLabel:       CloudServiceProvider,
	AksLabel:       CloudServiceProvider,
	EksctlLabel:    ClusterProvisioningTool,
	KopsLabel:      ClusterProvisioningTool,
	PoolNameLabel:  UnclassifiedProvider,
}

func GetProviderType(ln model.LabelName) NodeLabelProviderType {
	return nodeLabelToProviderType[ln]
}

var nodeGroupLabels = make(map[model.LabelName]bool)
var ngl []model.LabelName

type labelFeature struct {
	labelNames           []model.LabelName
	useDefault           bool
	openshiftClusterName string
	roleValues           []model.LabelValue
}

type queryFeature struct {
	query   string
	onLabel model.LabelName
}

func (lf *labelFeature) Type() featureType {
	return labelType
}

func (lf *labelFeature) NodeGroupQueryFeatures() (qfs []*queryFeature) {
	switch {
	case len(lf.roleValues) > 0:
		for _, rv := range lf.roleValues {
			qfs = append(qfs, &queryFeature{
				query:   fmt.Sprintf(`%s{%s="%v"}`, NodeRoleMetric, common.Role, rv),
				onLabel: common.Role,
			})
		}
	case len(lf.labelNames) > 0:
		for _, ln := range lf.labelNames {
			var query, metric string
			switch ln {
			case NonExistingLabel:
				metric = NodeInfoMetric
			default:
				metric = NodeLabelMetric
			}
			if lf.useDefault {
				sln := string(ln)
				query = common.LabelReplaceArbitraryValue(metric+common.Braces, sln, string(DefaultNodeGroup), sln, common.EmptyRegex)
			} else {
				query = fmt.Sprintf(`%s{%s=~"%v"}`, metric, ln, common.HasValue.String())
			}
			qfs = append(qfs, &queryFeature{
				query:   query,
				onLabel: ln,
			})
		}
	}
	return
}

func labelNameCmp(lna, lnb model.LabelName) int {
	return cmp.Compare(GetProviderType(lna), GetProviderType(lnb))
}

func detectNameLabel(cluster string, result model.Matrix) {
	var foundDefault bool
	merged := make(model.Metric)
	for _, ss := range result {
		maps.Copy(merged, ss.Metric)
		// make sure we can use DefaultNodeGroup
		values := maps.Values(ss.Metric)
		for _, value := range values {
			if foundDefault = value == DefaultNodeGroup; foundDefault {
				break
			}
		}
	}
	lns := maps.Keys(merged)
	llns := len(lns)
	if llns > 1 {
		slices.SortStableFunc(lns, labelNameCmp)
		allowedLabels := []model.LabelName{lns[0]}
		for i := 1; i < llns; i++ {
			var allowed bool
			for j := 0; j < i; j++ {
				if allowed = allowedLowerLabelsProviders[GetProviderType(lns[j])][GetProviderType(lns[i])]; allowed {
					allowedLabels = append(allowedLabels, lns[i])
				}
			}
		}
		lns = allowedLabels
		llns = len(lns)
	}
	if llns > 0 {
		clusterFeatures[cluster] = &labelFeature{
			labelNames: lns,
			useDefault: !foundDefault && llns < 2,
		}
	}
	/*else if len(node.ClusterNodeRoles[cluster]) > 0 {
		clusterFeatures[cluster] = &labelFeature{
			roleValues: maps.Keys(node.ClusterNodeRoles[cluster]),
		}
	} else {
		clusterFeatures[cluster] = &labelFeature{
			labelNames: []model.LabelName{NonExistingLabel},
			useDefault: true,
		}
	}*/
}
