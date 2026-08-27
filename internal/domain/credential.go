package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"
)

type EvidenceTask struct {
	TaskID          string           `json:"taskID"`
	Reason          string           `json:"reason"`
	RequiredActions string           `json:"requiredActions"`
	AssignedTo      string           `json:"assignedTo"`
	Status          ResamplingStatus `json:"status"`
}

type EvidenceSnapshot struct {
	TreeID              string         `json:"treeID"`
	Species             string         `json:"species"`
	LocationDescription string         `json:"locationDescription"`
	ProtectedStatus     bool           `json:"protectedStatus"`
	BaselineVersion     int            `json:"baselineVersion"`
	BatchID             string         `json:"batchID"`
	CollectedAt         string         `json:"collectedAt"`
	TargetTissues       []string       `json:"targetTissues"`
	TargetQuantity      int            `json:"targetQuantity"`
	SampleID            string         `json:"sampleID"`
	Label               string         `json:"label"`
	ActualQuantity      int            `json:"actualQuantity"`
	ContainerCondition  string         `json:"containerCondition"`
	ChainNotes          string         `json:"chainNotes"`
	QualityStatus       QualityStatus  `json:"qualityStatus"`
	ReviewNote          string         `json:"reviewNote"`
	Tasks               []EvidenceTask `json:"tasks"`
}

type PreservationFreeze struct {
	FreezeID       string           `json:"freezeID"`
	BatchID        string           `json:"batchID"`
	EvidenceDigest string           `json:"evidenceDigest"`
	FrozenBy       string           `json:"frozenBy"`
	FrozenAt       time.Time        `json:"frozenAt"`
	CredentialID   string           `json:"credentialID"`
	Snapshot       EvidenceSnapshot `json:"snapshot"`
}
type SpecimenCredential struct {
	CredentialID  string    `json:"credentialID"`
	BatchID       string    `json:"batchID"`
	IssuedAt      time.Time `json:"issuedAt"`
	Issuer        string    `json:"issuer"`
	PayloadDigest string    `json:"payloadDigest"`
	Status        string    `json:"status"`
}

func BuildEvidenceSnapshot(tree TreeRecord, batch SamplingBatch, inspection SampleInspection, tasks []ResamplingTask) EvidenceSnapshot {
	tissues := append([]string{}, batch.TargetTissues...)
	sort.Strings(tissues)
	ordered := append([]ResamplingTask{}, tasks...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].TaskID < ordered[j].TaskID })
	states := make([]EvidenceTask, 0, len(ordered))
	for _, t := range ordered {
		states = append(states, EvidenceTask{TaskID: t.TaskID, Reason: t.Reason, RequiredActions: t.RequiredActions, AssignedTo: t.AssignedTo, Status: t.Status})
	}
	return EvidenceSnapshot{TreeID: tree.TreeID, Species: tree.Species, LocationDescription: tree.LocationDescription, ProtectedStatus: tree.ProtectedStatus, BaselineVersion: tree.BaselineVersion, BatchID: batch.BatchID, CollectedAt: batch.CollectedAt.UTC().Format(time.RFC3339Nano), TargetTissues: tissues, TargetQuantity: batch.TargetQuantity, SampleID: inspection.SampleID, Label: inspection.Label, ActualQuantity: inspection.Quantity, ContainerCondition: inspection.ContainerCondition, ChainNotes: inspection.ChainNotes, QualityStatus: inspection.QualityStatus, ReviewNote: inspection.ReviewNote, Tasks: states}
}

func SnapshotDigest(snapshot EvidenceSnapshot) string {
	payload, _ := json.Marshal(snapshot)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func EvidenceDigest(tree TreeRecord, batch SamplingBatch, inspection SampleInspection, tasks []ResamplingTask) string {
	return SnapshotDigest(BuildEvidenceSnapshot(tree, batch, inspection, tasks))
}
func NewCredential(id, batchID, issuer, digest string, now time.Time) SpecimenCredential {
	return SpecimenCredential{CredentialID: id, BatchID: batchID, IssuedAt: now, Issuer: issuer, PayloadDigest: digest, Status: "valid"}
}
func VerifyCredential(c SpecimenCredential, expected string) (bool, string) {
	if c.Status != "valid" {
		return false, "凭据状态无效"
	}
	if c.PayloadDigest != expected {
		return false, "证据摘要不匹配"
	}
	return true, "凭据有效"
}
