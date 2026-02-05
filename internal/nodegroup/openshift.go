package nodegroup

import (
	"fmt"
	"strings"

	"github.com/densify-dev/container-data-collection/internal/common"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"golang.org/x/exp/maps"
)

type openshiftFeature struct {
	clusterName              model.LabelValue
	detectedMachineSets      map[model.LabelValue]bool
	machineSets              []model.LabelValue
	machineSetsSubstitutions map[model.LabelValue]model.LabelValue
}

func (of *openshiftFeature) Type() featureType {
	return openshiftType
}

const (
	labelMachineSet           = "machine_set"
	labelMachineSetFull       = "machine_set_full"
	labelOpenshiftCluster     = "openshift_cluster_name"
	master                    = "master"
	openshiftMasterNodeRegex  = "^(.+)-master.*"
	machineSetQueryFmt        = "avg(mapi_machine_set_status_replicas{}) by (%s)"
	nodeMachineSetQueryFormat = `avg(label_replace(label_replace(label_replace(%s{}, "%s", "$1", "%s", "^(.*?)(?:-[a-z0-9]{5}-\\d+|-[a-z0-9]{5}|-\\d+)$"), "%s", "$1", "%s", "^(.{58}).{5}$"), "%s", "$1", "%s", "^(?:%s)-(.*)")) by (%s, %s, %s)`
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
	numOpenShiftClusters := n
	// detect actual machine sets names (this is worker nodes only, the master nodes have no machine test)
	query = fmt.Sprintf(machineSetQueryFmt, common.Name)
	if _, err = common.CollectAndProcessMetric(query, promRange, detectOpenshiftMachineSets); err != nil {
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
	// We first need to get the cluster name (in MAPI), so we can trim it from the machine node name. We
	// do that by finding the master nodes, which are guaranteed to have `-master` immediately after
	// the cluster name.
	query = fmt.Sprintf(`avg(label_replace(%s{%s=~"%s"},"%s", "$1", "%s", "%s")) by (%s);`,
		NodeInfoMetric, common.Node, openshiftMasterNodeRegex, labelOpenshiftCluster, common.Node, openshiftMasterNodeRegex, labelOpenshiftCluster)
	if n, err = common.CollectAndProcessMetric(query, promRange, detectOpenshiftClusterName); err != nil || n == 0 {
		// error already handled
		return
	}
	clusterNames := make([]string, numOpenShiftClusters)
	for _, cf := range clusterFeatures {
		if of, ok := toOpenShiftFeature(cf); ok && of.clusterName != common.Empty {
			clusterNames = append(clusterNames, string(of.clusterName))
		}
	}
	clusterNamesRegex := strings.Join(clusterNames, common.Or)
	query = fmt.Sprintf(nodeMachineSetQueryFormat, NodeInfoMetric, labelMachineSetFull, common.Node, labelMachineSetFull, labelMachineSetFull, labelMachineSet, labelMachineSetFull, clusterNamesRegex, common.Node, labelMachineSetFull, labelMachineSet)
	if n, err = common.CollectAndProcessMetric(query, promRange, createMachineSets); err != nil {
		// error already handled
		return
	}
	return
}

func detectOpenshiftMachines(cluster string, result model.Matrix) {
	if len(result) > 0 {
		_, _ = ensureFeature(cluster)
	}
}

func detectOpenshiftClusterName(cluster string, result model.Matrix) {
	if of, ok := ensureFeature(cluster); ok {
		for _, ss := range result {
			clusterName := ss.Metric[labelOpenshiftCluster]
			if of.clusterName == common.Empty {
				of.clusterName = clusterName
			} else if of.clusterName != clusterName {
				err := fmt.Errorf("openshift cluster has two distinct cluster names in master node names: %q and %q", of.clusterName, clusterName)
				common.LogError(err, common.DefaultLogFormat, cluster, common.NodeGroupEntityKind)
				delete(clusterFeatures, cluster)
				return
			}
		}
	}
}

func detectOpenshiftMachineSets(cluster string, result model.Matrix) {
	if of, ok := ensureFeature(cluster); ok {
		for _, ss := range result {
			if machineSetName, f := ss.Metric[common.Name]; f {
				of.detectedMachineSets[machineSetName] = true
			}
		}
	}
}

func createMachineSets(cluster string, result model.Matrix) {
	if of, ok := ensureFeature(cluster); ok {
		if _, ok = nodeGroups[cluster]; !ok {
			if l := result.Len(); l > 0 {
				nodeGroups[cluster] = make(map[string]*nodeGroup, l)
			}
		}
		detectedMachineSets := maps.Keys(of.detectedMachineSets)
		for _, ss := range result {
			var process bool
			var nodeGroupName string
			if machineSetName, f := ss.Metric[labelMachineSet]; f {
				nodeGroupName = string(machineSetName)
				if of.detectedMachineSets[machineSetName] || machineSetName == master {
					of.machineSets = append(of.machineSets, machineSetName)
					process = true
				} else if len(machineSetName) == 58 {
					// check for truncation
					for _, dms := range detectedMachineSets {
						if strings.HasPrefix(string(dms), string(machineSetName)) {
							of.machineSets = append(of.machineSets, machineSetName)
							of.machineSetsSubstitutions = map[model.LabelValue]model.LabelValue{machineSetName: dms}
							process = true
							break
						}
					}
				}
			}
			if process {
				nodeName := string(ss.Metric[common.Node])
				if _, f := nodeGroups[cluster][nodeGroupName]; !f {
					nodeGroups[cluster][nodeGroupName] = &nodeGroup{
						// currentSize is initialized to 0!
						cpuLimit:    common.UnknownValue,
						cpuRequest:  common.UnknownValue,
						cpuCapacity: common.UnknownValue,
						memLimit:    common.UnknownValue,
						memRequest:  common.UnknownValue,
						memCapacity: common.UnknownValue,
						labelMap:    make(map[string]string),
					}
				}
				if !strings.Contains(nodeGroups[cluster][nodeGroupName].nodes, nodeName) {
					nodeGroups[cluster][nodeGroupName].nodes = nodeGroups[cluster][nodeGroupName].nodes + nodeName + common.Or
					nodeGroups[cluster][nodeGroupName].currentSize++
				}
			}
		}
	}
}

func ensureFeature(cluster string) (of *openshiftFeature, f bool) {
	var cf clusterFeature
	if cf, f = clusterFeatures[cluster]; f {
		of, f = toOpenShiftFeature(cf)
	} else {
		of = &openshiftFeature{}
		clusterFeatures[cluster] = of
		f = true
	}
	return
}
