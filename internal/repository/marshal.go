package repository

import (
	"biocuration/internal/domain"
	"encoding/json"
)

func encodeTissues(v []string) string { b, _ := json.Marshal(v); return string(b) }
func decodeTissues(v string) []string {
	var out []string
	_ = json.Unmarshal([]byte(v), &out)
	return out
}
func statusLabel(s domain.BatchStatus) string { return string(s) }
