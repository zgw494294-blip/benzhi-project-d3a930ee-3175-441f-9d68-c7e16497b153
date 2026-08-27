package application

import (
	"biocuration/internal/domain"
)

func ensureKey(key string) error {
	if key == "" {
		return &domain.DomainError{Code: domain.ErrInvalidInput, Message: "幂等键不能为空"}
	}
	return nil
}
