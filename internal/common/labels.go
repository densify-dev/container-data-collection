package common

import (
	"fmt"
)

type LabelReplaceCondition int

const (
	HasValue LabelReplaceCondition = iota
	Always
	EmptyRegex
)

const (
	hasValueStr         = ".+"
	alwaysStr           = ".*"
	emptyStr            = "^$"
	firstCapturingGroup = "$1"
)

func (lrc LabelReplaceCondition) String() (s string) {
	switch lrc {
	case HasValue:
		s = hasValueStr
	case Always:
		s = alwaysStr
	case EmptyRegex:
		s = emptyStr
	}
	return
}

func (lrc LabelReplaceCondition) makeRegexValueMap() map[string]bool {
	re := lrc.String()
	return map[string]bool{
		re:                    true,
		Wrap(re, Parenthesis): true,
	}
}

var HasValueMap = HasValue.makeRegexValueMap()
var AlwaysMap = Always.makeRegexValueMap()
var EmptyMap = EmptyRegex.makeRegexValueMap()

func LabelReplace(query, dstLabel, srcLabel string, lrc LabelReplaceCondition) string {
	return LabelReplaceArbitraryValue(query, dstLabel, firstCapturingGroup, srcLabel, lrc)
}

func LabelReplaceArbitraryValue(query, dstLabel, dstValue, srcLabel string, lrc LabelReplaceCondition) string {
	return fmt.Sprintf(`label_replace(%s, "%s", "%s", "%s", "(%s)")`, query, dstLabel, dstValue, srcLabel, lrc.String())
}
