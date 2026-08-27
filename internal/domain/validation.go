package domain

import "strings"

func NormalizeText(v string) string { return strings.Join(strings.Fields(strings.TrimSpace(v)), " ") }
func ValidateActor(v string) error {
	if NormalizeText(v) == "" {
		return invalid("操作人不能为空")
	}
	if len([]rune(v)) > 80 {
		return invalid("操作人长度超过限制")
	}
	return nil
}
func ValidateID(v string) error {
	if NormalizeText(v) == "" {
		return invalid("标识不能为空")
	}
	if len(v) > 120 {
		return invalid("标识长度超过限制")
	}
	return nil
}
