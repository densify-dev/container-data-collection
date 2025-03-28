package common

import (
	"fmt"
	"github.com/prometheus/common/model"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FileType int

const (
	Config FileType = iota
	Attributes
)

func (ft FileType) String() (s string) {
	switch ft {
	case Config:
		s = configFileName
	case Attributes:
		s = attributesFileName
	}
	return
}

const (
	rootFolder         = "data"
	fileExt            = ".csv"
	configFileName     = "config"
	attributesFileName = "attributes"
	dirPerm            = 0755
)

var entityKinds = []string{ClusterEntityKind, NodeEntityKind, NodeGroupEntityKind, ContainerEntityKind, Hpa, RqEntityKind, CrqEntityKind}

func MkdirAll() error {
	for _, cluster := range ClusterNames {
		for _, entityKind := range entityKinds {
			if err := os.MkdirAll(filepath.Join(rootFolder, cluster, entityKind), dirPerm); err != nil {
				return err
			}
		}
	}
	return nil
}

func GetFileName(cluster, entityKind, fileName string) string {
	return filepath.Join(rootFolder, cluster, entityKind, fileName+fileExt)
}

func GetFileNameByType(cluster, entityKind string, ft FileType) string {
	return GetFileName(cluster, entityKind, ft.String())
}

func GetExtraFileNameByType(cluster, entityKind string, ft FileType) string {
	return GetFileName(cluster, entityKind, SnakeCase(entityKind, Extra, ft.String()))
}

func FormatTime(mt model.Time) string {
	t := mt.Time()
	return Format(&t)
}

func FormatTimeInSec(i int64) string {
	t := time.Unix(i, 0)
	return Format(&t)
}

func FormatCurrentTime() string {
	return Format(&CurrentTime)
}

func Format(t *time.Time) string {
	return t.Format(time.RFC3339Nano)
}

func ReplaceColons(s string) string {
	return strings.ReplaceAll(s, colon, Dot)
}

func ReplaceSemiColons(s string) string {
	return strings.ReplaceAll(s, semicolonStr, Dot)
}

func ReplaceSemiColonsPipes(s string) string {
	return strings.ReplaceAll(s, semicolonStr, Or)
}

func GetCsvHeaderFormat(entityKind string, subject string) (format string, f bool) {
	ek := strings.ToLower(entityKind)
	format, f = csvHeaderFormats[subject][ek]
	return
}

type ValueFunc[T Number] func(T) bool

func KnownValueFunc[T Number](n T) bool {
	switch any(n).(type) {
	case float32, float64:
		return float64(n) != UnknownValueFloat
	default:
		return int(n) != UnknownValue
	}
}

func PositiveValueFunc[T Number](n T) bool {
	return n > 0
}

func PrintCSVNumberValue[T Number](file *os.File, value T, last bool) error {
	return PrintCSVNumberValueConditional(file, value, last, KnownValueFunc)
}

func PrintCSVPositiveNumberValue[T Number](file *os.File, value T, last bool) error {
	return PrintCSVNumberValueConditional(file, value, last, PositiveValueFunc)
}

func PrintCSVNumberValueConditional[T Number](file *os.File, value T, last bool, f ValueFunc[T]) error {
	sVal := ""
	if f(value) {
		var frmt string
		switch any(value).(type) {
		case float32, float64:
			frmt = "%f"
		default:
			frmt = "%d"
		}
		sVal = fmt.Sprintf(frmt, value)
	}
	return PrintCSVStringValue(file, sVal, last)
}

func PrintCSVTimeValue(file *os.File, value *time.Time, last bool) error {
	sVal := ""
	if !(value == nil || value.IsZero()) {
		sVal = Format(value)
	}
	return PrintCSVStringValue(file, sVal, last)
}

func PrintCSVStringValue(file *os.File, value string, last bool) (err error) {
	s := "," + value
	if last {
		s += lf
	}
	_, err = fmt.Fprintf(file, "%s", s)
	return
}

func PrintCSVLabelMap(file *os.File, labelMap map[string]string, last bool) error {
	return ConditionalPrintCSVLabelMap(file, labelMap, last, nil)
}

func ConditionalPrintCSVLabelMap(file *os.File, labelMap map[string]string, last bool, rejectKeys map[string]bool) (err error) {
	keys := SortedKeySet(labelMap)
	for _, key := range keys {
		if reject := rejectKeys[key]; reject {
			continue
		}
		var maxValueLen int
		if lkey := len(key); lkey >= maxKeyLen {
			continue
		} else {
			maxValueLen = maxKeyLen + 3 - lkey
		}
		value := labelMap[key]
		value = strings.ReplaceAll(strings.ReplaceAll(value, Comma, Space), doubleQuote, Empty)
		if len(value) > maxValueLen {
			value = value[:maxValueLen]
		}
		if _, err = fmt.Fprintf(file, "%s : %s%s", key, value, Or); err != nil {
			return
		}
	}
	if last {
		_, err = fmt.Fprintf(file, lf)
	}
	return
}

var (
	containerEntityKindName    = JoinComma(CamelCase(Entity, Name), CamelCase(Entity, Type), CamelCase(ContainerEntityKind, Name))
	containerHpaEntityKindName = JoinComma(containerEntityKindName, CamelCase(Hpa, Name))
)

type headerBuilder struct {
	entityKindName     string
	includeClusterName bool
	includeNamespace   bool
}

func (hb *headerBuilder) generateCsvHeaderFormat(subject string) string {
	l := 3
	if hb.includeClusterName {
		l++
	}
	if hb.includeNamespace {
		l++
	}
	components := make([]string, l)
	if hb.includeClusterName {
		components[0] = CamelCase(ClusterEntityKind, Name)
	}
	if hb.includeNamespace {
		components[1] = CamelCase(Namespace)
	}
	components[l-3] = hb.entityKindName
	components[l-2] = CamelCase(subject, Time)
	components[l-1] = "%s\n"
	return JoinComma(components...)
}

var headerBuilders = map[string]*headerBuilder{
	ClusterEntityKind:   {entityKindName: CamelCase(Name)},
	NodeEntityKind:      {entityKindName: CamelCase(NodeEntityKind, Name), includeClusterName: true},
	NodeGroupEntityKind: {entityKindName: CamelCase(NodeGroupEntityKind, Name), includeClusterName: true},
	RqEntityKind:        {entityKindName: CamelCase(RqEntityKind, Name), includeClusterName: true, includeNamespace: true},
	CrqEntityKind:       {entityKindName: CamelCase(CrqEntityKind, Name), includeClusterName: true},
	ContainerEntityKind: {entityKindName: containerEntityKindName, includeClusterName: true, includeNamespace: true},
	HpaEntityKind:       {entityKindName: containerHpaEntityKindName, includeClusterName: true, includeNamespace: true},
}

var csvHeaderFormats = makeCsvHeaderFormats()

var headerFormatSubjects = []string{Metric, Event}

func makeCsvHeaderFormats() map[string]map[string]string {
	formats := make(map[string]map[string]string, len(headerFormatSubjects))
	for _, subject := range headerFormatSubjects {
		m := make(map[string]string, len(headerBuilders))
		for entityKind, hb := range headerBuilders {
			m[entityKind] = hb.generateCsvHeaderFormat(subject)
		}
		formats[subject] = m
	}
	return formats
}
