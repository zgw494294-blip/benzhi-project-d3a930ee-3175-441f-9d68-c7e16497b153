package domain

import "time"

type TreeRecord struct {
	TreeID              string    `json:"treeID"`
	Species             string    `json:"species"`
	LocationDescription string    `json:"locationDescription"`
	ProtectedStatus     bool      `json:"protectedStatus"`
	BaselineVersion     int       `json:"baselineVersion"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

func NewTree(id, species, location string, protected bool, now time.Time) (TreeRecord, error) {
	id = NormalizeText(id)
	species = NormalizeText(species)
	location = NormalizeText(location)
	if id == "" || species == "" || location == "" {
		return TreeRecord{}, invalid("treeID、species、locationDescription 不能为空")
	}
	if err := ValidateID(id); err != nil {
		return TreeRecord{}, err
	}
	return TreeRecord{TreeID: id, Species: species, LocationDescription: location, ProtectedStatus: protected, BaselineVersion: 1, CreatedAt: now, UpdatedAt: now}, nil
}

func (t TreeRecord) UpdateBaseline(treeID, species, location string, protected bool, expectedVersion int, now time.Time) (TreeRecord, bool, error) {
	treeID = NormalizeText(treeID)
	species = NormalizeText(species)
	location = NormalizeText(location)
	if treeID == "" || species == "" || location == "" {
		return t, false, invalid("treeID、species、locationDescription 不能为空")
	}
	if treeID != t.TreeID {
		return t, false, invalid("treeID 与路径不一致")
	}
	if expectedVersion <= 0 {
		return t, false, invalid("expectedBaselineVersion 必须大于零")
	}
	if expectedVersion != t.BaselineVersion {
		return t, false, versionError("expectedBaselineVersion 不匹配")
	}
	if species == t.Species && location == t.LocationDescription && protected == t.ProtectedStatus {
		return t, false, nil
	}
	t.Species = species
	t.LocationDescription = location
	t.ProtectedStatus = protected
	t.BaselineVersion++
	t.UpdatedAt = now
	return t, true, nil
}

func (t TreeRecord) ValidateBatch(target []string, quantity int) error {
	if len(target) == 0 {
		return invalid("targetTissues 不能为空")
	}
	if quantity <= 0 {
		return invalid("targetQuantity 必须大于零")
	}
	if t.ProtectedStatus && quantity > 5 {
		return invalid("保护树木单批次目标数量不得超过 5")
	}
	allowed := map[string]bool{"leaf": true, "bark": true, "branch": true, "root": true, "fruit": true}
	for _, tissue := range target {
		if !allowed[tissue] {
			return invalid("存在不支持的目标组织")
		}
	}
	return nil
}
