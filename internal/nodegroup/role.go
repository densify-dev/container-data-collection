package nodegroup

import (
	"errors"
	"fmt"
	"strings"

	"github.com/densify-dev/container-data-collection/internal/common"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

const (
	roleQuery      = NodeRoleMetric + common.Braces
	byRoleQueryFmt = `%s{%s%s"%s"}`
	roleClauseFmt  = " or (%s unless on (%s) %s)"
)

type roleFeature struct {
}

func (rf *roleFeature) Type() featureType {
	return roleType
}

func (rf *roleFeature) NodeAndGroupCoreQueryFmt() (s string) {
	if rf != nil {
		s = roleEng.coreQuery
	}
	return
}

func (rf *roleFeature) LabelNames() []model.LabelName {
	return []model.LabelName{common.Role}
}

func (rf *roleFeature) AdjustNodeGroupName(name string) string {
	return name
}

func determineRoleFeatures(promRange *v1.Range) (err error) {
	var n int
	query := fmt.Sprintf("avg(%s) by (%s)", roleQuery, common.Role)
	if n, err = common.CollectAndProcessMetric(query, promRange, detectRoles); err != nil || n == 0 {
		// error already handled
		return
	}
	if roleEng != nil && roleEng.queryForRole {
		_, err = common.CollectAndProcessMetric(roleEng.coreQuery, promRange, createRoles)
	}
	return
}

type roleEngine struct {
	configuredRoles    []string
	configuredRolesMap map[string]bool
	coreQuery          string
	queryForRole       bool
}

var roleEng *roleEngine

func ensureRoleEngine() *roleEngine {
	if roleEng == nil {
		configuredRoles := strings.Split(common.Params.Collection.RoleList, common.Comma)
		if len(configuredRoles) > 0 {
			configuredRolesMap := make(map[string]bool, len(configuredRoles))
			for _, role := range configuredRoles {
				configuredRolesMap[role] = true
			}
			coreQuery := buildCoreQuery(configuredRoles)
			roleEng = &roleEngine{
				configuredRoles:    configuredRoles,
				configuredRolesMap: configuredRolesMap,
				coreQuery:          coreQuery,
			}
		}
	}
	return roleEng
}

func detectRoles(cluster string, result model.Matrix) {
	if len(result) > 0 {
		var err error
		if ensureRoleEngine() == nil {
			err = errors.New("bad role configuration")
		}
		if err == nil {
			var missingRoles []string
			for _, ss := range result {
				if roleName, f1 := ss.Metric[common.Role]; f1 {
					rName := string(roleName)
					if !roleEng.configuredRolesMap[rName] {
						missingRoles = append(missingRoles, rName)
					}
				}
			}
			if len(missingRoles) > 1 {
				err = fmt.Errorf("more than one role detected and not configured, cannot determine order: %s", strings.Join(missingRoles, common.Comma))
			}
		}
		if err == nil {
			clusterFeatures[cluster] = &roleFeature{}
			roleEng.queryForRole = true
		} else {
			common.LogError(err, common.DefaultLogFormat, cluster, common.NodeGroupEntityKind)
		}
	}
}

func buildCoreQuery(roleValues []string) string {
	var sb strings.Builder
	prevRoles := roleValues[0]
	op := common.ExactEqual
	sb.WriteString(subRoleQuery(op, prevRoles))
	for i := 1; i < len(roleValues); i++ {
		if i > 1 {
			op = common.RegexMatch
		}
		sb.WriteString(roleQueryClause(common.ExactEqual, roleValues[i], op, prevRoles))
		prevRoles += common.Or + roleValues[i]
	}
	sb.WriteString(roleQueryClause(common.NotRegexMatch, prevRoles, op, prevRoles))
	return sb.String()
}

func subRoleQuery(op, value string) string {
	return fmt.Sprintf(byRoleQueryFmt, NodeRoleMetric, common.Role, op, value)
}

func roleQueryClause(roleOp, role, prevRolesOp, prevRoles string) string {
	return fmt.Sprintf(roleClauseFmt, subRoleQuery(roleOp, role), common.Node, subRoleQuery(prevRolesOp, prevRoles))
}

func createRoles(cluster string, result model.Matrix) {
	if _, f := ensureRoleFeature(cluster); f {
		createNodeGroup(cluster, result, common.Role)
	}
}
