package nodegroup

import (
	"fmt"

	"github.com/densify-dev/container-data-collection/internal/common"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type openshiftFeature struct {
	clusterName              string
	detectedMachineSets      map[string]bool
	machineSetsSubstitutions map[string]string
}

func (of *openshiftFeature) Type() featureType {
	return openshiftType
}

func (of *openshiftFeature) NodeAndGroupCoreQueryFmt() string {
	return openshiftCoreQueryFmt
}

func (of *openshiftFeature) LabelNames() []model.LabelName {
	return []model.LabelName{labelMachineSet}
}

func (of *openshiftFeature) AdjustNodeGroupName(name string) (s string) {
	if fullName, f := of.machineSetsSubstitutions[name]; f {
		s = fullName
	} else {
		s = name
	}
	return
}

const (
	maxNameLength             = 63
	randomSuffixLength        = 5
	maxPrefixLength           = maxNameLength - randomSuffixLength
	labelMachineSet           = "machine_set"
	labelOpenshiftCluster     = "openshift_cluster_name"
	openshiftMasterNodeRegex1 = "^(.+-master).*"
	openshiftMasterNodeRegex2 = "^(.+)-master.*"
	masterNodeQueryFmt        = `label_replace(label_replace(%s{%s=~"%s"}, "%s", "$1", "%s", "%s"), "%s", "$1", "%s", "%s")`
	machineSetQueryFmt        = `label_replace(mapi_machine_set_status_replicas{}, "%s", "$1", "%s", "^(.{0,%d}).*")`
	nodeMachineSetQueryFormat = `label_replace(label_replace(%s{}, "%s", "$1", "%s", "^(.*?)(?:-[a-z0-9]{%d}-\\d+|-[a-z0-9]{%d}|-\\d+)$"), "%s", "$1", "%s", "^(.{%d}).{%d}$")`
)

var (
	openshiftCoreQueryFmt = fmt.Sprintf(nodeMachineSetQueryFormat, NodeInfoMetric, common.DefaultFmt, common.Node,
		randomSuffixLength, randomSuffixLength, common.DefaultFmt, common.DefaultFmt, maxPrefixLength, randomSuffixLength)
)

func determineOpenshiftFeatures(promRange *v1.Range) (err error) {
	// This is the best query to detect OpenShift machines, as the master/control-plane machine set
	// is not created as a MachineSet CR, therefore `mapi_machineset_items > 0` does not detect it
	var n int
	query := "mapi_machine_items{} > 0"
	if n, err = common.CollectAndProcessMetric(query, promRange, detectOpenshiftMachines); err != nil || n == 0 {
		// error already handled
		return
	}
	// detect actual machine sets names (this is worker nodes only, the master nodes have no machine test)
	query = fmt.Sprintf(machineSetQueryFmt, common.NamePrefix, common.Name, maxPrefixLength)
	if _, err = common.CollectAndProcessMetric(query, promRange, detectOpenshiftMachineSets); err != nil {
		// error already handled
		return
	}
	query = fmt.Sprintf(masterNodeQueryFmt, NodeInfoMetric, common.Node, openshiftMasterNodeRegex1,
		labelMachineSet, common.Node, openshiftMasterNodeRegex1,
		labelOpenshiftCluster, labelMachineSet, openshiftMasterNodeRegex2)

	if _, err = common.CollectAndProcessMetric(query, promRange, detectMasterNodes); err != nil {
		// error already handled
		return
	}
	// The machine name is added as an annotation to the node, but the Openshift cluster monitoring
	// operators denies any annotations in its rollout of kube-state-metrics. We therefore have to
	// rely on the node name.
	// The machine name is also the node name, and its format is one of (depending on machine set
	// and potentially also OpenShift version):
	// 1. <cluster name>-<machine set name>-<number> (for master nodes)
	// 2. <cluster name>-<machine set name>-<5 character random string>-<number> (for master nodes)
	// 3. <cluster name>-<machine set name>-<5 character random string> (for worker nodes)
	// 4. As node name is limited by 63 character, if 3 yields a name which is too long, the machine set name is
	//    truncated and the 5-character string is added without a `-`:
	//    <cluster name>-<truncated machine set name><5 character random string> (for worker nodes)
	query = common.FormatRepeatedAuto(openshiftCoreQueryFmt, labelMachineSet)
	if n, err = common.CollectAndProcessMetric(query, promRange, createMachineSets); err != nil {
		// error already handled
		return
	}
	return
}

func detectOpenshiftMachines(cluster string, result model.Matrix) {
	if len(result) > 0 {
		_, _ = ensureOpenShiftFeature(cluster)
	}
}

func detectOpenshiftMachineSets(cluster string, result model.Matrix) {
	if of, ok := ensureOpenShiftFeature(cluster); ok {
		for _, ss := range result {
			if machineSetName, f := ss.Metric[common.Name]; f {
				msName := string(machineSetName)
				of.detectedMachineSets[msName] = true
				if machineSetNamePrefix, f2 := ss.Metric[model.LabelName(common.NamePrefix)]; f2 {
					msNamePrefix := string(machineSetNamePrefix)
					if fullMachineSetName, f3 := of.machineSetsSubstitutions[msNamePrefix]; f3 && fullMachineSetName != msName {
						err := fmt.Errorf("found two machine sets with same %d-character prefix %s: %s and %s; only %s will be used", maxPrefixLength, msNamePrefix, fullMachineSetName, msName, fullMachineSetName)
						common.LogError(err, common.DefaultLogFormat, cluster, common.NodeGroupEntityKind)
					} else {
						of.machineSetsSubstitutions[msNamePrefix] = msName
					}
				}
			}
		}
	}
}

func detectMasterNodes(cluster string, result model.Matrix) {
	if of, ok := ensureOpenShiftFeature(cluster); ok {
		for _, ss := range result {
			if clusterName, f1 := ss.Metric[labelOpenshiftCluster]; f1 {
				cName := string(clusterName)
				if of.clusterName == common.Empty {
					of.clusterName = cName
					// add the master "machine set" (even though it is not a machine set in Openshift)
					if masterMachineSetName, f2 := ss.Metric[labelMachineSet]; f2 {
						mmsName := string(masterMachineSetName)
						of.detectedMachineSets[mmsName] = true
						of.machineSetsSubstitutions[mmsName] = mmsName
					}
				} else if of.clusterName != cName {
					err := fmt.Errorf("two openshift cluster names found: %v and %v", of.clusterName, clusterName)
					common.LogError(err, common.DefaultLogFormat, cluster, common.NodeGroupEntityKind)
				}
			}
		}
	}
}

func createMachineSets(cluster string, result model.Matrix) {
	if _, f := ensureOpenShiftFeature(cluster); f {
		createNodeGroup(cluster, result, labelMachineSet)
	}
}
