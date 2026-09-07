package pool

import "testing"

// item is a Resettable used to exercise Pool: it tracks whether Reset was
// called and how many values newFunc had to allocate.
type item struct {
	value int
	reset bool
}

func (i *item) Reset() {
	i.value = 0
	i.reset = true
}

func TestPool_GetCreatesWhenEmpty(t *testing.T) {
	created := 0
	p := New(func() *item {
		created++
		return &item{}
	})

	v := p.Get()
	if v == nil {
		t.Fatal("Get returned nil")
	}
	if created != 1 {
		t.Fatalf("newFunc called %d times, want 1", created)
	}
}

func TestPool_PutResetsBeforeReuse(t *testing.T) {
	p := New(func() *item { return &item{} })

	v := p.Get()
	v.value = 42
	p.Put(v)

	if !v.reset {
		t.Fatal("Put did not call Reset on the returned value")
	}

	got := p.Get()
	if got.value != 0 {
		t.Fatalf("value reused with stale state: got.value = %d, want 0", got.value)
	}
}

func TestPool_NilNewFunc_GetReturnsZeroValue(t *testing.T) {
	p := New[*item](nil)

	v := p.Get()
	if v != nil {
		t.Fatalf("Get() = %v, want nil (the zero value of *item)", v)
	}
}

func TestPool_PutMakesValueReusable(t *testing.T) {
	created := 0
	p := New(func() *item {
		created++
		return &item{}
	})

	v := p.Get()
	p.Put(v)
	_ = p.Get()

	if created != 1 {
		t.Fatalf("newFunc called %d times, want 1 (value should have been reused)", created)
	}
}
