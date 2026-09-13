package dynamodb

import (
	"context"
	"testing"
)

func TestIDAllocatorLeasesBlockOnce(t *testing.T) {
	client, fake := testClient(t)
	alloc := NewIDAllocator(client, "us-east-1", 2)
	ctx := context.Background()
	first, err := alloc.Lease(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first != 0 {
		t.Fatalf("first id = %d", first)
	}
	updates := 0
	for _, target := range fake.Targets {
		if len(target) >= 10 && target[len(target)-10:] == "UpdateItem" {
			updates++
		}
	}
	if updates != 1 {
		t.Fatalf("UpdateItem count after first lease = %d", updates)
	}
	second, err := alloc.Lease(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if second != 1 {
		t.Fatalf("second id = %d", second)
	}
	updates = 0
	for _, target := range fake.Targets {
		if len(target) >= 10 && target[len(target)-10:] == "UpdateItem" {
			updates++
		}
	}
	if updates != 1 {
		t.Fatalf("in-block lease issued extra UpdateItem (%d)", updates)
	}
	before := fake.RequestCount()
	third, err := alloc.Lease(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if third != 2 {
		t.Fatalf("third id = %d", third)
	}
	if fake.RequestCount() != before+1 {
		t.Fatalf("exhausted block should issue exactly one UpdateItem, requests %d -> %d", before, fake.RequestCount())
	}
}

func TestIDAllocatorDefaultBlockSize(t *testing.T) {
	client, _ := testClient(t)
	alloc := NewIDAllocator(client, "", 0)
	if alloc.blockSize != defaultBlockSize || alloc.region != defaultRegion {
		t.Fatalf("defaults = %+v", alloc)
	}
}
