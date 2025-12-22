package core

import (
	"sort"
	"sync"
)

const (
	minForwardBufferSize = 4 * 1024
	midForwardBufferSize = 16 * 1024
	maxPooledBufferSize  = 64 * 1024
)

// HierarchicalBufferPool provides reusable buffers with multiple size classes.
// It avoids retaining oversized buffers indefinitely by refusing to pool buffers larger than maxPooledBufferSize.
type HierarchicalBufferPool struct {
	classes []int
	pools   map[int]*sync.Pool
}

func NewHierarchicalBufferPool(maxSize int) *HierarchicalBufferPool {
	maxSize = normalizeForwardBufferSize(maxSize)

	classes := []int{minForwardBufferSize}
	if maxSize >= midForwardBufferSize {
		classes = append(classes, midForwardBufferSize)
	}
	pooledMax := maxSize
	if pooledMax > maxPooledBufferSize {
		pooledMax = maxPooledBufferSize
	}
	if pooledMax > midForwardBufferSize {
		classes = append(classes, pooledMax)
	}
	if maxSize > pooledMax {
		// Allow growth beyond pooled sizes, but do not pool these oversized buffers.
		classes = append(classes, maxSize)
	}
	classes = uniqueSortedInts(classes)

	pools := make(map[int]*sync.Pool, len(classes))
	for _, size := range classes {
		if size > maxPooledBufferSize {
			continue
		}
		sz := size
		pools[sz] = &sync.Pool{
			New: func() any {
				b := make([]byte, sz)
				return &b
			},
		}
	}

	return &HierarchicalBufferPool{
		classes: classes,
		pools:   pools,
	}
}

func (p *HierarchicalBufferPool) Get(size int) *[]byte {
	size = normalizeForwardBufferSize(size)
	if cls, ok := p.pools[size]; ok {
		v := cls.Get()
		if b, ok := v.(*[]byte); ok && b != nil {
			return b
		}
	}
	b := make([]byte, size)
	return &b
}

func (p *HierarchicalBufferPool) Put(buf *[]byte) {
	if buf == nil || *buf == nil {
		return
	}
	sz := cap(*buf)
	if sz <= 0 || sz > maxPooledBufferSize {
		return
	}
	if cls, ok := p.pools[sz]; ok {
		cls.Put(buf)
	}
}

func (p *HierarchicalBufferPool) InitialSize() int {
	return p.classes[0]
}

func (p *HierarchicalBufferPool) NextSize(cur int) (int, bool) {
	cur = normalizeForwardBufferSize(cur)
	for i := 0; i < len(p.classes)-1; i++ {
		if p.classes[i] == cur {
			return p.classes[i+1], true
		}
	}
	return cur, false
}

func normalizeForwardBufferSize(size int) int {
	if size <= 0 {
		return midForwardBufferSize
	}
	if size < minForwardBufferSize {
		return minForwardBufferSize
	}
	// Round to 4KiB boundaries for predictable pooling.
	size = (size + minForwardBufferSize - 1) / minForwardBufferSize * minForwardBufferSize
	if size < minForwardBufferSize {
		return minForwardBufferSize
	}
	return size
}

func uniqueSortedInts(in []int) []int {
	if len(in) == 0 {
		return in
	}
	sort.Ints(in)
	out := make([]int, 0, len(in))
	last := -1
	for _, v := range in {
		if v != last {
			out = append(out, v)
			last = v
		}
	}
	return out
}
