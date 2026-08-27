package selfcheck

import (
	"fmt"
	"net/http"
)

func AssertStatus(got, want int) error {
	if got != want {
		return fmt.Errorf("HTTP 状态码 %d，期望 %d", got, want)
	}
	return nil
}
func AssertHealth(c *http.Client, url string) error {
	r, e := c.Get(url + "/healthz")
	if e != nil {
		return e
	}
	defer r.Body.Close()
	return AssertStatus(r.StatusCode, 200)
}
