package common

import (
	cconf "github.com/densify-dev/container-config/config"
	"strings"
	"time"
)

const (
	Version = "4.0.0"
)

// globals
var Params *cconf.Parameters
var CurrentTime time.Time

func SetCurrentTime() {
	t := time.Now().UTC()
	switch Params.Collection.Interval {
	case Days:
		CurrentTime = time.Date(t.Year(), t.Month(), t.Day()-Params.Collection.OffsetInt, 0, 0, 0, 0, t.Location())
	case Hours:
		CurrentTime = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()-Params.Collection.OffsetInt, 0, 0, 0, t.Location())
	default:
		CurrentTime = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute()-Params.Collection.OffsetInt, 0, 0, t.Location())
	}

}

// AddToLabelMap is used to add values to label map used for attributes
func AddToLabelMap(key string, value string, labelPath map[string]string) {
	if _, ok := labelPath[key]; !ok {
		value = strings.ReplaceAll(strings.ReplaceAll(value, lf, Empty), cr, Empty)
		if len(value) > maxLabelValueLen {
			labelPath[key] = value[:maxLabelValueLen]
		} else {
			labelPath[key] = value
		}
		return
	}
	if strings.Contains(value, semicolonStr) {
		currValue := Empty
		for _, l := range value {
			currValue = currValue + string(l)
			if l == semicolon {
				AddToLabelMap(key, currValue[:len(currValue)-1], labelPath)
				currValue = Empty
			}
		}
		AddToLabelMap(key, currValue, labelPath)
		return
	}
	currValue := Empty
	notPresent := true
	for _, l := range labelPath[key] {
		currValue = currValue + string(l)
		if l == semicolon {
			if currValue[:len(currValue)-1] == value {
				notPresent = false
				break
			}
			currValue = Empty
		}
	}
	if currValue != value && notPresent {
		if len(value) > maxLabelValueLen {
			labelPath[key] = labelPath[key] + semicolonStr + value[:maxLabelValueLen]
		} else {
			labelPath[key] = labelPath[key] + semicolonStr + value
		}
	}
}
