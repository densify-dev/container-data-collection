package common

import "strings"

const (
	snakedK8s = "k_8_s"
)

func K8sSnakeCase(elements ...string) string {
	elems := append([]string{K8s}, elements...)
	snaked := SnakeCase(elems...)
	return strings.ReplaceAll(snaked, snakedK8s, K8s)
}

// semconv labels
var (
	SemconvNamespaceName = K8sSnakeCase(Namespace, Name)
	SemconvKind          = K8sSnakeCase(Kind)
	SemconvOwnerName     = K8sSnakeCase(Owner, Name)
	SemconvContainerName = K8sSnakeCase(Container, Name)
	SemconvNodeName      = K8sSnakeCase(Node, Name)
	TelemetrySdkLanguage = SnakeCase(Telemetry, Sdk, Language)
)
