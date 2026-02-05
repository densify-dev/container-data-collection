package nodegroup

type featureType string

const (
	labelType     featureType = "label"
	openshiftType featureType = "openshift"
	roleType      featureType = "role"
)

type clusterFeature interface {
	Type() featureType
}

var clusterFeatures = make(map[string]clusterFeature)

func toLabelFeature(cf clusterFeature) (lf *labelFeature, ok bool) {
	if cf.Type() == labelType {
		lf, ok = cf.(*labelFeature)
	}
	return
}

func toOpenShiftFeature(cf clusterFeature) (of *openshiftFeature, ok bool) {
	if cf.Type() == openshiftType {
		of, ok = cf.(*openshiftFeature)
	}
	return
}
