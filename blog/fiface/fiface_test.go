package fiface

import "testing"

func TestFifaceCompare(t *testing.T) {
	// Two no-op impls; comparing is safe
	c1 := New("c1", Config{})

	c2 := New("c2", Config{})

	if c1 == c2 {
		t.Fail()
	}
}
