package model

import "github.com/leorolland/matchergen/examples/types"

//go:generate matchergen -type IDCard -output ../modeltest/id_card_matcher.go .
type IDCard struct {
	firstName string
	id        types.UUID
}

func (i *IDCard) FirstName() string {
	return i.firstName
}

func (i *IDCard) ID() types.UUID {
	return i.id
}
