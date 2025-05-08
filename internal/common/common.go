package common

import (
	_ "embed"
	cconf "github.com/densify-dev/container-config/config"
	"strings"
	"time"
)

// globals
//
//go:generate sh -c "printf %s $(git describe --abbrev=0 --tags) > version.txt"
//go:embed version.txt
var version string
var Version = strings.TrimSpace(version)
var Params *cconf.Parameters
var CurrentTime time.Time
var Interval time.Duration
var Step time.Duration

func SetCurrentTime() {
	t := time.Now().UTC()
	Interval = time.Duration(Params.Collection.IntervalSize)
	switch Params.Collection.Interval {
	case Days:
		CurrentTime = time.Date(t.Year(), t.Month(), t.Day()-Params.Collection.OffsetInt, 0, 0, 0, 0, t.Location())
		Interval *= time.Hour * 24
	case Hours:
		CurrentTime = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()-Params.Collection.OffsetInt, 0, 0, 0, t.Location())
		Interval *= time.Hour
	default:
		CurrentTime = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute()-Params.Collection.OffsetInt, 0, 0, t.Location())
		Interval *= time.Minute
	}
	Step = time.Minute * time.Duration(Params.Collection.SampleRate)
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
