package common

import (
	"fmt"
	cconf "github.com/densify-dev/container-config/config"
	"github.com/prometheus/common/model"
	"strings"
)

type labelFilter struct {
	labelNames string
	labels     string
}

type ClusterFilter struct {
	spec   *cconf.ClusterFilterParameters
	filter *labelFilter
}

func NewClusterFilter(cfp *cconf.ClusterFilterParameters) *ClusterFilter {
	cf := &ClusterFilter{spec: cfp, filter: &labelFilter{}}
	cf.finalize()
	return cf
}

type Result struct {
	Query  string
	Matrix model.Matrix
	Error  error
}

type ClusterResultMap map[string]*Result

func NumClusters() int {
	return len(filtersByName)
}

func Eval(n int, eval bool) int {
	if eval {
		return n
	} else {
		return NumClusters() - n
	}
}

var BoolValues = []bool{true, false}

func FoundCounter(n int) (res []bool) {
	for _, b := range BoolValues {
		if Eval(n, b) > 0 {
			res = append(res, b)
		}
	}
	return
}

func Found(indicators map[string]int, indicator string, eval bool) bool {
	return Eval(indicators[indicator], eval) > 0
}

func FoundIndicatorCounter(indicators map[string]int, indicator string) []bool {
	return FoundCounter(indicators[indicator])
}

func RegisterClusterFilters(cfps []*cconf.ClusterFilterParameters) error {
	for _, cfp := range cfps {
		if err := registerClusterFilter(NewClusterFilter(cfp)); err != nil {
			return err
		}
	}
	for _, qlf := range labelFilters {
		qlf.finalize()
	}
	return nil
}

func registerClusterFilter(cf *ClusterFilter) error {
	if err := cf.validate(); err != nil {
		return err
	}
	for _, filter := range filtersByName {
		if err := filter.validateDistinct(cf); err != nil {
			return err
		}
	}
	filtersByName[cf.spec.Name] = cf
	lns := KeySet(cf.spec.Identifiers)
	fp := fingerprint(lns)
	var qlf *queryLabelFilter
	var found bool
	if qlf, found = labelFilters[fp]; !found {
		qlf = &queryLabelFilter{labelNames: lns, filter: &labelFilter{}}
		labelFilters[fp] = qlf
	}
	qlf.clusterFilters = append(qlf.clusterFilters, cf)
	return nil
}

type queryLabelFilter struct {
	labelNames     model.LabelNames
	clusterFilters []*ClusterFilter
	filter         *labelFilter
}

var queryPerCluster = true
var filtersByName = make(map[string]*ClusterFilter)
var noIdentifiersFilter bool
var labelFilters = make(map[model.Fingerprint]*queryLabelFilter)

func (cf *ClusterFilter) validate() (err error) {
	if cf == nil || cf.spec == nil || cf.spec.Name == Empty {
		err = fmt.Errorf("nil cluster or cluster with no name")
	} else if len(cf.spec.Identifiers) == 0 {
		// cf.Identifiers may be nil or Empty, but only if we have a single filter
		noIdentifiersFilter = true
	}
	return
}

func (cf *ClusterFilter) validateDistinct(other *ClusterFilter) (err error) {
	if noIdentifiersFilter {
		err = fmt.Errorf("a cluster filter with no identifiers is not distinct")
	} else if cf.spec.Name == other.spec.Name {
		err = fmt.Errorf("cluster filter with name %s already configured", cf.spec.Name)
	}
	if err == nil {
		for ln, lv := range other.spec.Identifiers {
			if Contains(cf.spec.Identifiers, ln, lv) {
				err = fmt.Errorf("cluster filter with name %s already contains label %s = %s", cf.spec.Name, ln, lv)
				break
			}
		}
	}
	return
}

func fingerprint(ln model.LabelNames) model.Fingerprint {
	return emptyValueLabelSet(ln).Fingerprint()
}

func emptyValueLabelSet(ln model.LabelNames) model.LabelSet {
	ls := make(model.LabelSet, len(ln))
	for _, name := range ln {
		ls[name] = Empty
	}
	return ls
}

func split(result *Result, cluster string, cfs []*ClusterFilter) (crm ClusterResultMap) {
	if cluster == Empty {
		crm = make(ClusterResultMap, len(cfs))
		for _, cf := range cfs {
			r := &Result{Error: result.Error, Query: result.Query}
			for _, ss := range result.Matrix {
				if IsSubset(ss.Metric, cf.spec.Identifiers) {
					r.Matrix = append(r.Matrix, ss)
				}
			}
			crm[cf.spec.Name] = r
		}
	} else {
		crm = make(ClusterResultMap, 1)
		crm[cluster] = result
	}
	return
}

func (cf *ClusterFilter) finalize() {
	cf.filter.calculateFilter(cf.spec.Identifiers)
}

func (qlf *queryLabelFilter) finalize() {
	lss := make([]model.LabelSet, len(qlf.clusterFilters))
	for i, cf := range qlf.clusterFilters {
		lss[i] = cf.spec.Identifiers
	}
	qlf.filter.calculateFilter(lss...)
}

// calculateFilters assumes that all LabelSets share exactly the same model.LabelNames as keys
func (lf *labelFilter) calculateFilter(lss ...model.LabelSet) {
	if lf == nil {
		return
	}
	l := len(lss)
	var op string
	switch l {
	case 0:
		return
	case 1:
		op = exactEqual
	default:
		op = regexMatch
	}
	ks := SortedKeySet(lss[0])
	n := len(ks)
	lnfs := make([]string, n)
	lfs := make([]string, n)
	for i, ln := range ks {
		vals := make([]string, l)
		for j, ls := range lss {
			vals[j] = string(ls[ln])
		}
		lfn := string(ln)
		lnfs[i] = lfn
		lfs[i] = lfn + op + doubleQuote + Join(Or, vals...) + doubleQuote
	}
	lf.labelNames = JoinComma(lnfs...)
	lf.labels = JoinComma(lfs...)
	return
}

func (qlf *queryLabelFilter) adjustQuery(query string) (queries map[string]string) {
	if queryPerCluster {
		queries = make(map[string]string, len(qlf.clusterFilters))
		for _, cf := range qlf.clusterFilters {
			q := cf.filter.replacePlaceholders(query)
			queries[cf.spec.Name] = q
		}
	} else {
		queries = make(map[string]string, 1)
		q := qlf.filter.replacePlaceholders(query)
		// Empty key means need to filter the result per cluster
		queries[Empty] = q
	}
	return
}

func (lf *labelFilter) replacePlaceholders(query string) (q string) {
	if lf == nil {
		q = query
	} else {
		q = strings.ReplaceAll(query, labelsPlaceholder, lf.labels)
		q = strings.ReplaceAll(q, LabelNamesPlaceholder, lf.labelNames)
	}
	q = cleanQuery(q)
	return
}

func cleanQuery(query string) string {
	// we may have superfluous commas in the form of "{,", ",}", "(,", ",)", or ",,"
	s := strings.ReplaceAll(query, commaComma, Comma)
	s = strings.ReplaceAll(s, leftBraceComma, leftBrace)
	s = strings.ReplaceAll(s, commaRightBrace, rightBrace)
	s = strings.ReplaceAll(s, leftBracketComma, leftBracket)
	s = strings.ReplaceAll(s, commaRightBracket, rightBracket)
	// this may leave us with empty selectors in the form of "()" or empty label selectors in the form of "{}"
	s = strings.ReplaceAll(s, emptyByClause, Empty)
	s = strings.ReplaceAll(s, emptyLabelSelector, Empty)
	return s

}

func (qlf *queryLabelFilter) filterValue(cluster, query string, value model.Value, err error) ClusterResultMap {
	var mat model.Matrix
	var e error
	switch v := value.(type) {
	case model.Matrix:
		mat = v
		if mat.Len() == 0 {
			e = fmt.Errorf("no data returned, model.Matrix is empty")
		}
	default:
		e = fmt.Errorf("cannot filter model.Value of type %T by cluster", v)
	}
	// if passed original error, use it
	if err != nil {
		e = err
	}
	return split(&Result{Query: query, Matrix: mat, Error: e}, cluster, qlf.clusterFilters)
}
