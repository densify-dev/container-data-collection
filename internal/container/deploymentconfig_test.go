package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cconf "github.com/densify-dev/container-config/config"
	"github.com/densify-dev/container-data-collection/internal/common"
	"github.com/prometheus/common/model"
)

func isolateOwnerships(t *testing.T) {
	t.Helper()
	oldNamespaces, oldOwnerships := namespaces, ownerships
	oldDetectedOwners, oldTypes := detectedOwners, detectedOwnershipTypes
	oldParams := common.Params
	common.Params = &cconf.Parameters{Prometheus: &cconf.PrometheusParameters{UrlConfig: &cconf.UrlConfig{}}, Collection: &cconf.CollectionParameters{}}
	namespaces = make(map[string]map[string]*namespace)
	ownerships = make(map[string]clusterOwnerships)
	detectedOwners = make(map[string]map[string]bool)
	detectedOwnershipTypes = make(map[string]map[string]map[string]bool)
	t.Cleanup(func() {
		namespaces, ownerships = oldNamespaces, oldOwnerships
		detectedOwners, detectedOwnershipTypes = oldDetectedOwners, oldTypes
		common.Params = oldParams
		common.UnregisterClusterQueryExclusion(noReplicationControllerOwners)
	})
}

func ownershipSample(ns, label, name, kind, owner string) *model.SampleStream {
	return &model.SampleStream{Metric: model.Metric{
		"namespace": model.LabelValue(ns), model.LabelName(label): model.LabelValue(name),
		"owner_kind": model.LabelValue(kind), "owner_name": model.LabelValue(owner),
	}}
}

func containerSample(ns, pod string) *model.SampleStream {
	return &model.SampleStream{Metric: model.Metric{
		"namespace": model.LabelValue(ns), "pod": model.LabelValue(pod), "container": "main",
	}}
}

func TestSecondLevelOwnershipAndWorkloadIdentity(t *testing.T) {
	for _, test := range []struct {
		name, intermediate, ownerKind, finalKind string
		th                                       *typeHolder
	}{
		{"deploymentconfig", "ReplicationController", "DeploymentConfig", common.DeploymentConfig, rcth},
		{"other-rc-owner", "ReplicationController", "OtherController", "othercontroller", rcth},
		{"deployment", "ReplicaSet", "Deployment", common.Deployment, rsth},
		{"cronjob", "Job", "CronJob", common.CronJob, jth},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateOwnerships(t)
			for _, cluster := range []string{"cluster-a", "cluster-b"} {
				for _, ns := range []string{"app-a", "app-b"} {
					pods := model.Matrix{
						ownershipSample(ns, "pod", "pod-1", test.intermediate, "revision-1"),
						ownershipSample(ns, "pod", "pod-2", test.intermediate, "revision-2"),
						ownershipSample(ns, "pod", "standalone-pod", test.intermediate, "standalone"),
					}
					pth.getOwners(cluster, pods)
					test.th.getOwners(cluster, model.Matrix{
						ownershipSample(ns, test.th.getTypeLabelName(), "revision-1", test.ownerKind, "workload"),
						ownershipSample(ns, test.th.getTypeLabelName(), "revision-2", test.ownerKind, "workload"),
					})
					addContainerAndOwners(cluster, model.Matrix{
						containerSample(ns, "pod-1"), containerSample(ns, "pod-2"),
						containerSample(ns, "standalone-pod"), containerSample(ns, "unowned-pod"),
					})
					objects := namespaces[cluster][ns].objects
					if len(objects) != 3 || objects[key(test.finalKind, "workload", ns)] == nil {
						t.Fatalf("unexpected top-level objects: %v", objects)
					}
					for _, pod := range []string{"pod-1", "pod-2"} {
						_, producer, ok := getContainer(cluster, containerSample(ns, pod))
						if !ok || producer.obj.name != "workload" || producer.obj.kind != test.finalKind {
							t.Fatalf("%s did not resolve to %s/workload", pod, test.finalKind)
						}
					}
					if _, _, ok := getContainer(cluster, containerSample(ns, "standalone-pod")); !ok {
						t.Fatal("standalone owner lost its container")
					}
					th := &typeHolder{}
					if _, ok := th.containerFields(cluster, []string{ns, "revision-1", test.intermediate, "main"}); ok {
						t.Fatal("direct-owner variant accepted a second-level-owned object")
					}
					fields, ok := th.containerFields(cluster, []string{ns, "workload", test.ownerKind, "main"})
					if !ok || fields[2] != getOwnerKindValue(test.finalKind) {
						t.Fatalf("top-level workload fields = %v, %v", fields, ok)
					}
					if _, ok := th.containerFields(cluster, []string{ns, "standalone", test.intermediate, "main"}); !ok {
						t.Fatal("standalone direct-owner variant rejected")
					}
				}
			}
		})
	}
}

func TestReplicationControllerQueryGates(t *testing.T) {
	isolateOwnerships(t)
	detectedOwnershipTypes[common.ReplicationController] = map[string]map[string]bool{
		"empty": {}, "dc": {common.DeploymentConfig: true}, "other": {"othercontroller": true},
	}
	for _, cluster := range []string{"missing", "empty", "dc", "other"} {
		if got := excludeReplicationControllerQueries(cluster, replicationControllerGroupClause); got != (cluster == "missing" || cluster == "empty") {
			t.Errorf("RC query exclusion for %s = %v", cluster, got)
		}
		for _, metric := range []string{"labels", "created", "metadata_generation", "spec_replicas", "spec_strategy_rollingupdate_max_surge", "spec_strategy_rollingupdate_max_unavailable"} {
			if got := excludeReplicationControllerQueries(cluster, "openshift_deploymentconfig_"+metric+"{}"); got != (cluster != "dc") {
				t.Errorf("DC %s query exclusion for %s = %v", metric, cluster, got)
			}
		}
		if excludeReplicationControllerQueries(cluster, "kube_replicationcontroller_spec_replicas{}") ||
			excludeReplicationControllerQueries(cluster, replicaSetGroupClause) {
			t.Fatal("exclusion changed an existing query")
		}
	}
	for _, subject := range []string{common.Metric, common.Event} {
		clauses := buildGroupClauses(subject)
		found := false
		for clause, builder := range clauses {
			if strings.Contains(clause, replicationControllerOwnerMetric) {
				found = true
				if builder.lnt != fullOwnerLabelNames || builder.th == nil {
					t.Fatal("RC variant lacks owner identity processor")
				}
				if strings.HasSuffix(clause, "container)") != (subject == common.Metric) {
					t.Fatalf("incorrect suffix for %s: %s", subject, clause)
				}
			}
		}
		if !found {
			t.Fatalf("RC variant missing for %s", subject)
		}
		delete(detectedOwnershipTypes[common.ReplicationController], "dc")
		delete(detectedOwnershipTypes[common.ReplicationController], "other")
		for clause := range buildGroupClauses(subject) {
			if strings.Contains(clause, replicationControllerOwnerMetric) {
				t.Fatal("empty inner maps enabled RC variant")
			}
		}
		detectedOwnershipTypes[common.ReplicationController]["dc"] = map[string]bool{common.DeploymentConfig: true}
	}
}

func TestReplicationControllerRediscoveryAndMalformedOwners(t *testing.T) {
	isolateOwnerships(t)
	const cluster, ns = "cluster", "app"
	pth.getOwners(cluster, model.Matrix{ownershipSample(ns, "pod", "pod", "ReplicationController", "rc")})
	valid := ownershipSample(ns, "replicationcontroller", "rc", "DeploymentConfig", "dc")
	rcth.getOwners(cluster, model.Matrix{valid})
	addContainerAndOwners(cluster, model.Matrix{containerSample(ns, "pod")})
	resetReplicationControllerOwners()
	if !excludeReplicationControllerQueries(cluster, replicationControllerGroupClause) {
		t.Fatal("rediscovery retained previous detection")
	}
	for _, sample := range (model.Matrix{
		ownershipSample(ns, "replicationcontroller", "rc", "DeploymentConfig", ""),
		ownershipSample(ns, "replicationcontroller", "rc", "DeploymentConfig", "<none>"),
		ownershipSample(ns, "replicationcontroller", "rc", "", "dc"),
		ownershipSample(ns, "wrong_label", "rc", "DeploymentConfig", "dc"),
	}) {
		rcth.getOwners(cluster, model.Matrix{sample})
	}
	addContainerAndOwners(cluster, model.Matrix{containerSample(ns, "pod")})
	_, producer, ok := getContainer(cluster, containerSample(ns, "pod"))
	if !ok || producer.obj.kind != common.ReplicationController {
		t.Fatal("missing owner data did not fall back to direct RC owner")
	}
	if !excludeReplicationControllerQueries(cluster, replicationControllerGroupClause) {
		t.Fatal("malformed owners enabled RC queries")
	}
	rcth.getOwners(cluster, model.Matrix{valid})
	if excludeReplicationControllerQueries(cluster, replicationControllerGroupClause) {
		t.Fatal("newly discovered owner did not enable RC queries")
	}
}

func TestDeploymentConfigAttributesAndCurrentSize(t *testing.T) {
	isolateOwnerships(t)
	t.Chdir(t.TempDir())
	const cluster, ns = "cluster", "app"
	if err := os.MkdirAll(filepath.Join("data", cluster, common.ContainerEntityKind), 0755); err != nil {
		t.Fatal(err)
	}
	oldWriters := containerWorkloadWriters
	containerWorkloadWriters = common.NewWorkloadWriters()
	containerWorkloadWriters.AddMetricWorkloadWriters(common.CurrentSize)
	t.Cleanup(func() {
		containerWorkloadWriters.CloseAndClearWorkloadWriters(common.ContainerEntityKind)
		containerWorkloadWriters = oldWriters
	})
	pth.getOwners(cluster, model.Matrix{
		ownershipSample(ns, "pod", "pod", "ReplicationController", "rc"),
		ownershipSample(ns, "pod", "standalone-pod", "ReplicationController", "standalone"),
	})
	rcth.getOwners(cluster, model.Matrix{ownershipSample(ns, "replicationcontroller", "rc", "DeploymentConfig", "dc")})
	addContainerAndOwners(cluster, model.Matrix{containerSample(ns, "pod"), containerSample(ns, "standalone-pod")})
	obj := namespaces[cluster][ns].objects[key(common.DeploymentConfig, "dc", ns)]
	if obj.currentSize != common.UnknownValue || len(obj.labelMap) != 0 || !obj.createTime.IsZero() {
		t.Fatal("missing OSM metrics fabricated attributes")
	}
	sample := &model.SampleStream{
		Metric: model.Metric{"namespace": ns, "deploymentconfig": "dc", "label_app": "example"},
		Values: []model.SamplePair{{Timestamp: model.Time(100000), Value: 1}},
	}
	dcth.getObjectMetricString(cluster, model.Matrix{sample})
	omh := &objectMetricHolder{metricHolder: &metricHolder{}, typeHolder: dcth}
	for _, metric := range []string{maxSurge, maxUnavailable, metadataGeneration} {
		omh.metric = metric
		omh.getObjectMetric(cluster, model.Matrix{sample})
		if obj.labelMap[metric] != "1" {
			t.Fatalf("%s = %q", metric, obj.labelMap[metric])
		}
	}
	if obj.labelMap["label_app"] != "example" {
		t.Fatal("DC labels not collected")
	}
	omh.metric = createTime
	omh.getObjectMetric(cluster, model.Matrix{sample})
	if !obj.createTime.Equal(time.Unix(1, 0)) {
		t.Fatal("DC creation timestamp not collected")
	}
	omh.typeHolder = rcth
	rcSample := &model.SampleStream{
		Metric: model.Metric{"namespace": ns, "replicationcontroller": "rc"},
		Values: []model.SamplePair{{Value: 99}},
	}
	omh.getObjectMetric(cluster, model.Matrix{rcSample})
	if !obj.createTime.Equal(time.Unix(1, 0)) {
		t.Fatal("RC creation timestamp overwrote DC timestamp")
	}
	rcSample.Metric["replicationcontroller"] = "standalone"
	omh.getObjectMetric(cluster, model.Matrix{rcSample})
	if !namespaces[cluster][ns].objects[key(common.ReplicationController, "standalone", ns)].createTime.Equal(time.Unix(99, 0)) {
		t.Fatal("standalone RC creation timestamp not collected")
	}
	omh.metric = common.CurrentSizeName
	rcSample.Metric["replicationcontroller"] = "rc"
	omh.getObjectMetric(cluster, model.Matrix{rcSample})
	if obj.currentSize != common.UnknownValue {
		t.Fatal("owned RC replica count assigned to DC")
	}
	omh.typeHolder = dcth
	for _, size := range []model.SampleValue{3, 0} {
		sample.Values[0].Value = size
		omh.getObjectMetric(cluster, model.Matrix{sample})
		if obj.currentSize != int(size) {
			t.Fatalf("DC current size = %d, want %v", obj.currentSize, size)
		}
	}
}
