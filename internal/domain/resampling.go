package domain

import "time"

type ResamplingStatus string

const (
	ResamplingOpen   ResamplingStatus = "open"
	ResamplingClosed ResamplingStatus = "closed"
)

type ResamplingTask struct {
	TaskID          string           `json:"taskID"`
	BatchID         string           `json:"batchID"`
	Reason          string           `json:"reason"`
	RequiredActions string           `json:"requiredActions"`
	AssignedTo      string           `json:"assignedTo"`
	Status          ResamplingStatus `json:"status"`
	ResolvedAt      *time.Time       `json:"resolvedAt"`
}

func NewResampling(id string, inspection SampleInspection, assigned string) (ResamplingTask, error) {
	id = NormalizeText(id)
	assigned = NormalizeText(assigned)
	if id == "" || assigned == "" {
		return ResamplingTask{}, invalid("taskID 和 assignedTo 不能为空")
	}
	action := "补充来源链记录并重新采集，确认容器条件为 intact"
	return ResamplingTask{TaskID: id, BatchID: inspection.BatchID, Reason: inspection.ReviewNote, RequiredActions: action, AssignedTo: assigned, Status: ResamplingOpen}, nil
}
func (t *ResamplingTask) Resolve(now time.Time) error {
	if t.Status == ResamplingClosed {
		return stateError("补采任务已关闭")
	}
	t.Status = ResamplingClosed
	t.ResolvedAt = &now
	return nil
}
