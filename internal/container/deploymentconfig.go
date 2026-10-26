package container

import (
	"strings"

	"github.com/densify-dev/container-data-collection/internal/common"
)

const (
	replicationControllerOwnerMetric   = "kube_replicationcontroller_owner"
	replicationControllerOwnerSelector = replicationControllerOwnerMetric + `{owner_is_controller="true",owner_kind!="",owner_name!="",owner_name!="<none>"}`
	noReplicationControllerOwners      = "no-replicationcontroller-owners"
)

func excludeReplicationControllerQueries(cluster, query string) bool {
	owners := detectedOwnershipTypes[common.ReplicationController][cluster]
	if strings.Contains(query, "openshift_deploymentconfig_") {
		return !owners[common.DeploymentConfig]
	}
	return strings.Contains(query, replicationControllerOwnerMetric) && len(owners) == 0
}

func resetReplicationControllerOwners() {
	// Discovery must run even if the previous collection found no owned RCs.
	common.UnregisterClusterQueryExclusion(noReplicationControllerOwners)
	delete(detectedOwnershipTypes, common.ReplicationController)
	for _, owners := range ownerships {
		for ownedKey, owner := range owners {
			if strings.HasPrefix(ownedKey, common.ReplicationController+idSep) {
				delete(owners, ownedKey)
			} else if owner.directOwner.kind == common.ReplicationController {
				owner.topLevelOwner = nil
			}
		}
	}
}
