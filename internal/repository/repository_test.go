package repository

import (
	"biocuration/internal/domain"
	"context"
	"testing"
	"time"
)

func TestStoreRoundTrip(t *testing.T) {
	s, e := Open(":memory:")
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	now := time.Now()
	tree, _ := domain.NewTree("t", "sp", "loc", false, now)
	if e = s.SaveTree(context.Background(), tree); e != nil {
		t.Fatal(e)
	}
	got, e := s.Tree(context.Background(), "t")
	if e != nil || got.TreeID != "t" {
		t.Fatalf("读取失败 %v %#v", e, got)
	}
	if e = s.Integrity(context.Background()); e != nil {
		t.Fatal(e)
	}
}
