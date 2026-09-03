// Package sample is fixture data for cmd/reset's tests: a mix of fields
// exercising every reset rule, plus a struct that must be left alone
// because it has no generate:reset marker.
package sample

// generate:reset
type Child struct {
	Count int
}

// generate:reset
type Value struct {
	Label string
}

// generate:reset
type Resettable struct {
	I     int
	Str   string
	Flag  bool
	StrP  *string
	S     []int
	M     map[string]string
	Child *Child
	Value Value
	Ch    chan int
}

// NotAnnotated has no marker comment, so it must not get a Reset() method.
type NotAnnotated struct {
	X int
}
