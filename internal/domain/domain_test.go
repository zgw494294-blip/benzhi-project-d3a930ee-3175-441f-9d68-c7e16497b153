package domain

import (
	"testing"
	"time"
)

func TestProtectedTreeLimit(t *testing.T) {
	tree, e := NewTree("t", "species", "loc", true, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if e = tree.ValidateBatch([]string{"leaf"}, 6); e == nil {
		t.Fatal("应拒绝超量采样")
	}
}
func TestInspectionQuality(t *testing.T) {
	i, e := Inspect("s", "b", "label", 1, "broken", "", time.Now())
	if e != nil || i.QualityStatus != QualityFail {
		t.Fatalf("质检应失败: %v %#v", e, i)
	}
	i, e = Inspect("s2", "b", "label", 1, "intact", "chain", time.Now())
	if e != nil || i.QualityStatus != QualityPass {
		t.Fatalf("质检应通过: %v %#v", e, i)
	}
}
func TestCredentialDigest(t *testing.T) {
	tree, _ := NewTree("t", "sp", "loc", false, time.Now())
	batch, _ := NewBatch("b", tree, "a", time.Now(), []string{"leaf"}, 1, time.Now())
	i, _ := Inspect("s", "b", "l", 1, "intact", "n", time.Now())
	d := EvidenceDigest(tree, batch, i, nil)
	c := NewCredential("c", "b", "issuer", d, time.Now())
	if ok, _ := VerifyCredential(c, d); !ok {
		t.Fatal("凭据应有效")
	}
}

func TestCollectionTimeWindowAndTimezone(t *testing.T) {
	valid, _ := time.Parse(time.RFC3339, "2024-01-02T05:00:00Z")
	if err := ValidateCollectionTime(valid); err != nil {
		t.Fatalf("窗口起点应有效: %v", err)
	}
	outside, _ := time.Parse(time.RFC3339, "2024-01-02T04:59:59Z")
	if err := ValidateCollectionTime(outside); err == nil {
		t.Fatal("窗口外时间应拒绝")
	}
	invalidZone, _ := time.Parse(time.RFC3339, "2024-01-02T12:00:00+23:00")
	if err := ValidateCollectionTime(invalidZone); err == nil {
		t.Fatal("无效时区应拒绝")
	}
}
