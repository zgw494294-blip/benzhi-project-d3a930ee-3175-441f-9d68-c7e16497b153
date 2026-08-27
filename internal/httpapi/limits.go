package httpapi

import (
	"biocuration/internal/domain"
	"io"
	"net/http"
)

func boundedBody(r *http.Request) io.Reader { return io.LimitReader(r.Body, 1<<20) }
func validPathPart(v string) error {
	if v == "" {
		return &domain.DomainError{Code: domain.ErrInvalidInput, Message: "路径参数不能为空"}
	}
	return nil
}
