package core

import (
	"sync"
	"testing"
)

func TestHierarchicalBufferPoolClasses(t *testing.T) {
	t.Parallel()

	p := NewHierarchicalBufferPool(32 * 1024)
	if len(p.classes) < 2 {
		t.Fatalf("expected multiple classes, got %v", p.classes)
	}
	if p.InitialSize() != 4*1024 {
		t.Fatalf("unexpected initial size: %d", p.InitialSize())
	}
	next, ok := p.NextSize(p.InitialSize())
	if !ok || next <= p.InitialSize() {
		t.Fatalf("expected next size > initial, got next=%d ok=%v", next, ok)
	}
}

func BenchmarkForwardingBufferPoolMixed(b *testing.B) {
	pool := NewHierarchicalBufferPool(32 * 1024)
	sizes := []int{4 * 1024, 16 * 1024, 32 * 1024}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sz := sizes[i%len(sizes)]
		bufPtr := pool.Get(sz)
		buf := *bufPtr
		buf[0] = 1
		pool.Put(bufPtr)
	}
}

func BenchmarkForwardingBufferPoolFixed32K(b *testing.B) {
	var fixed sync.Pool
	fixed.New = func() any {
		buf := make([]byte, 32*1024)
		return &buf
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bufPtr := fixed.Get().(*[]byte)
		buf := *bufPtr
		buf[0] = 1
		fixed.Put(bufPtr)
	}
}
