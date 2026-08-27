package domain

import (
	"strings"
	"time"
)

type BatchStatus string

const (
	StatusPendingInspection BatchStatus = "pending_inspection"
	StatusNeedsResampling   BatchStatus = "needs_resampling"
	StatusReadyToFreeze     BatchStatus = "ready_to_freeze"
	StatusFrozen            BatchStatus = "frozen"
)

type SamplingBatch struct {
	BatchID         string      `json:"batchID"`
	TreeID          string      `json:"treeID"`
	Collector       string      `json:"collector"`
	CollectedAt     time.Time   `json:"collectedAt"`
	TargetTissues   []string    `json:"targetTissues"`
	TargetQuantity  int         `json:"targetQuantity"`
	ExpectedVersion int         `json:"expectedVersion"`
	Status          BatchStatus `json:"status"`
	CreatedAt       time.Time   `json:"createdAt"`
}

func NewBatch(id string, tree TreeRecord, collector string, collectedAt time.Time, tissues []string, quantity int, now time.Time) (SamplingBatch, error) {
	id = NormalizeText(id)
	collector = NormalizeText(collector)
	if collector == "" {
		return SamplingBatch{}, invalid("collector 不能为空")
	}
	if err := ValidateActor(collector); err != nil {
		return SamplingBatch{}, err
	}
	if err := ValidateCollectionTime(collectedAt); err != nil {
		return SamplingBatch{}, err
	}
	normalizedTissues := make([]string, 0, len(tissues))
	for _, tissue := range tissues {
		normalizedTissues = append(normalizedTissues, strings.ToLower(NormalizeText(tissue)))
	}
	if err := PolicyFor(tree).Allows(normalizedTissues, quantity); err != nil {
		return SamplingBatch{}, err
	}
	return SamplingBatch{BatchID: id, TreeID: tree.TreeID, Collector: collector, CollectedAt: collectedAt, TargetTissues: normalizedTissues, TargetQuantity: quantity, ExpectedVersion: 1, Status: StatusPendingInspection, CreatedAt: now}, nil
}
func (b *SamplingBatch) MarkInspection(status QualityStatus) error {
	if b.Status == StatusFrozen {
		return stateError("已冻结批次不可再次质检")
	}
	if status == QualityPass {
		b.Status = StatusReadyToFreeze
	} else {
		b.Status = StatusNeedsResampling
	}
	b.ExpectedVersion++
	return nil
}
func (b *SamplingBatch) CloseResampling() error {
	if b.Status != StatusNeedsResampling {
		return stateError("当前批次没有待关闭的补采任务")
	}
	b.Status = StatusReadyToFreeze
	b.ExpectedVersion++
	return nil
}

func (b *SamplingBatch) RecordTaskResolution(allClosed bool) error {
	if b.Status != StatusNeedsResampling {
		return stateError("当前批次没有待关闭的补采任务")
	}
	if allClosed {
		b.Status = StatusReadyToFreeze
	}
	b.ExpectedVersion++
	return nil
}
func (b SamplingBatch) FreezeAllowed(tasks []ResamplingTask, inspection *SampleInspection) error {
	if b.Status == StatusFrozen {
		return stateError("批次已冻结")
	}
	if inspection == nil || inspection.QualityStatus != QualityPass {
		return stateError("批次尚未通过质检")
	}
	for _, t := range tasks {
		if t.Status != ResamplingClosed {
			return stateError("存在未关闭的补采任务")
		}
	}
	return nil
}
