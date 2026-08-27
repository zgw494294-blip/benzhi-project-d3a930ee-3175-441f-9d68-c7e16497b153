package main

import (
	"biocuration/internal/application"
	"biocuration/internal/httpapi"
	"biocuration/internal/repository"
	"biocuration/internal/selfcheck"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"time"
)

func call(c *http.Client, method, url string, body any, key string) (map[string]any, error) {
	var b bytes.Buffer
	if body != nil {
		json.NewEncoder(&b).Encode(body)
	}
	r, _ := http.NewRequest(method, url, &b)
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	res, e := c.Do(r)
	if e != nil {
		return nil, e
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var out map[string]any
	json.Unmarshal(raw, &out)
	if res.StatusCode >= 300 {
		return out, fmt.Errorf("%s: %s", res.Status, raw)
	}
	return out, nil
}
func main() {
	f, _ := os.CreateTemp("", "biocuration-selfcheck-*.db")
	defer os.Remove(f.Name())
	st, e := repository.Open(f.Name())
	if e != nil {
		panic(e)
	}
	defer st.Close()
	ts := httptest.NewServer(httpapi.New(application.New(st)).Handler())
	defer ts.Close()
	c := ts.Client()
	if e = selfcheck.AssertHealth(c, ts.URL); e != nil {
		panic(e)
	}
	// 使用固定且处于采样窗口内的时间，确保自检不受运行时刻影响。
	now := time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
	tree, e := call(c, "POST", ts.URL+"/v1/trees", map[string]any{"treeID": "tree-self", "species": "悬铃木", "locationDescription": "人民路 1 号", "protectedStatus": false}, "tree-1")
	if e != nil {
		panic(e)
	}
	_ = tree
	b, e := call(c, "POST", ts.URL+"/v1/trees/tree-self/batches", map[string]any{"collector": "采样员甲", "collectedAt": now, "targetTissues": []string{"leaf"}, "targetQuantity": 2}, "batch-1")
	if e != nil {
		panic(e)
	}
	batch := b
	bid := batch["batchID"].(string)
	ins, e := call(c, "POST", ts.URL+"/v1/batches/"+bid+"/inspection", map[string]any{"sampleID": "sample-1", "label": "L-001", "quantity": 2, "containerCondition": "broken", "chainNotes": ""}, "insp-1")
	if e != nil {
		panic(e)
	}
	tasks := ins["tasks"].([]any)
	task := tasks[0].(map[string]any)
	ver := int(batch["expectedVersion"].(float64)) + 1
	_, e = call(c, "POST", ts.URL+"/v1/batches/"+bid+"/resampling/resolve", map[string]any{"taskID": task["taskID"], "expectedVersion": 99}, "bad-version")
	if e == nil {
		panic("应拒绝版本冲突")
	}
	rb, e := call(c, "POST", ts.URL+"/v1/batches/"+bid+"/resampling/resolve", map[string]any{"taskID": task["taskID"], "expectedVersion": ver}, "resolve-1")
	if e != nil {
		panic(e)
	}
	b2 := rb["batch"].(map[string]any)
	_, e = call(c, "POST", ts.URL+"/v1/batches/"+bid+"/inspection", map[string]any{"sampleID": "sample-2", "label": "L-002", "quantity": 2, "containerCondition": "intact", "chainNotes": "补采来源链完整"}, "insp-2")
	if e != nil {
		panic(e)
	}
	var latest map[string]any
	latest, e = call(c, "POST", ts.URL+"/v1/batches/"+bid+"/inspection", map[string]any{"sampleID": "sample-3", "label": "L-003", "quantity": 2, "containerCondition": "intact", "chainNotes": "复核记录"}, "insp-3")
	if e != nil {
		panic(e)
	}
	b2 = latest["batch"].(map[string]any)
	ver2 := int(b2["expectedVersion"].(float64))
	fr, e := call(c, "POST", ts.URL+"/v1/batches/"+bid+"/freeze", map[string]any{"frozenBy": "复核员乙", "expectedVersion": ver2}, "freeze-1")
	if e != nil {
		panic(e)
	}
	cred := fr["credential"].(map[string]any)["credentialID"].(string)
	if cred == "" {
		panic(fmt.Sprintf("凭据响应异常 %#v", fr))
	}
	vr, e := call(c, "GET", ts.URL+"/v1/credentials/"+cred+"/verify", nil, "")
	if e != nil || vr["valid"] != true {
		panic(fmt.Sprintf("凭据验真失败: %#v %v", vr, e))
	}
	if _, e = call(c, "GET", ts.URL+"/v1/batches/"+bid+"/freeze", nil, ""); e != nil {
		panic(e)
	}
	if _, e = call(c, "GET", ts.URL+"/v1/batches/"+bid+"/resampling?status=closed&limit=10", nil, ""); e != nil {
		panic(e)
	}
	if _, e = call(c, "GET", ts.URL+"/v1/credentials/"+cred+"/verify?history=true&limit=10", nil, ""); e != nil {
		panic(e)
	}
	fmt.Println("selfcheck passed")
}
