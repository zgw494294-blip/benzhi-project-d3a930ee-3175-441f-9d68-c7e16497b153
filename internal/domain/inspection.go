package domain

import (
	"strings"
	"time"
)

type QualityStatus string

const (
	QualityPass QualityStatus = "pass"
	QualityFail QualityStatus = "fail"
)

type SampleInspection struct {
	SampleID           string        `json:"sampleID"`
	BatchID            string        `json:"batchID"`
	Label              string        `json:"label"`
	Quantity           int           `json:"quantity"`
	ContainerCondition string        `json:"containerCondition"`
	ChainNotes         string        `json:"chainNotes"`
	QualityStatus      QualityStatus `json:"qualityStatus"`
	ReviewNote         string        `json:"reviewNote"`
	RecordedAt         time.Time     `json:"recordedAt"`
}

func Inspect(sampleID, batchID, label string, quantity int, container, notes string, now time.Time) (SampleInspection, error) {
	sampleID = NormalizeText(sampleID)
	batchID = NormalizeText(batchID)
	label = NormalizeText(label)
	container = strings.ToLower(NormalizeText(container))
	notes = NormalizeText(notes)
	if sampleID == "" || batchID == "" || label == "" {
		return SampleInspection{}, invalid("sampleID、batchID、label 不能为空")
	}
	if quantity <= 0 {
		return SampleInspection{}, invalid("quantity 必须大于零")
	}
	status := QualityPass
	reasons := make([]string, 0, 2)
	if container != "intact" {
		reasons = append(reasons, "容器条件不完整")
	}
	if notes == "" {
		reasons = append(reasons, "来源链备注不完整")
	}
	review := ""
	if len(reasons) > 0 {
		status = QualityFail
		review = strings.Join(reasons, "；")
	}
	return SampleInspection{SampleID: sampleID, BatchID: batchID, Label: label, Quantity: quantity, ContainerCondition: container, ChainNotes: notes, QualityStatus: status, ReviewNote: review, RecordedAt: now}, nil
}

func ReconcileQuantity(actual, target int) error {
	if actual <= 0 {
		return invalid("quantity 必须大于零")
	}
	if actual > target {
		return invalid("quantity 超过批次 targetQuantity")
	}
	if actual < target {
		return invalid("quantity 不足批次 targetQuantity")
	}
	return nil
}
