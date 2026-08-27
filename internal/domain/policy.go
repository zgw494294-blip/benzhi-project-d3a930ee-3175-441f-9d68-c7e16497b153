package domain

import (
	"strings"
	"time"
)

type SamplingPolicy struct {
	Species                   string
	MaxQuantity               int
	RequiresIndependentReview bool
	AllowedTissues            []string
}

func PolicyFor(tree TreeRecord) SamplingPolicy {
	p := SamplingPolicy{Species: tree.Species, MaxQuantity: 20, AllowedTissues: []string{"leaf", "bark", "branch", "root", "fruit"}}
	if tree.ProtectedStatus {
		p.MaxQuantity = 5
		p.RequiresIndependentReview = true
	}
	return p
}
func (p SamplingPolicy) Allows(tissues []string, quantity int) error {
	if len(tissues) == 0 {
		return invalid("targetTissues 不能为空")
	}
	if quantity <= 0 || quantity > p.MaxQuantity {
		return invalid("采样数量不符合物种保护策略")
	}
	allowed := map[string]bool{}
	for _, v := range p.AllowedTissues {
		allowed[v] = true
	}
	for _, v := range tissues {
		v = strings.ToLower(NormalizeText(v))
		if !allowed[v] {
			return invalid("目标组织不在采样策略内")
		}
	}
	return nil
}
func CollectionWindow(t time.Time) bool {
	seconds := t.Hour()*3600 + t.Minute()*60 + t.Second()
	return seconds >= 5*3600 && seconds <= 21*3600
}
func ValidateCollectionTime(t time.Time) error {
	if t.IsZero() || t.Location() == nil {
		return invalid("采样时间不能为空")
	}
	_, offset := t.Zone()
	if offset < -14*3600 || offset > 14*3600 {
		return invalid("采样时间时区无效")
	}
	if !CollectionWindow(t) {
		return invalid("采样时间应在 05:00 至 21:00")
	}
	return nil
}
