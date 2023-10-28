package model

//go:generate matchergen -type Human -output ../modeltest/human_matcher.go .
type Human struct {
	name string
	age  int
}

func NewHuman(name string, age int) *Human {
	return &Human{name, age}
}

func (h *Human) Name() string {
	return h.name
}

func (h *Human) Age() int {
	return h.age
}
