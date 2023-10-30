# gomock matcher generator

Generate [gomock matcher](https://pkg.go.dev/github.com/golang/mock/gomock#Matcher) implementations from your models.

```shell
go install github.com/leorolland/matchergen
```

## How it works
1. Run `matchergen -Type Human ./mypackage`
2. Generator searches for all accessor methods of type `Human`, they will be used for the match assertion.
4. Generator creates a `HumanMatcher` which implements [gomock Matcher](https://pkg.go.dev/github.com/golang/mock/gomock#Matcher) that can be instanciated with `NewHumanMatcher(expectedName, expectedAge)`. The matcher can be passed to gomocks `.EXPECT()` methods that expect a `Human` as a parameter.
5. gomock calls `Matches` on the generated matcher, which validates that all the accessors return the expected value (which are passed in `NewHumanMatcher`)

## Usage

1. Add `//go:generate matchergen` to your model:
(try in `/examples` directory)
```go
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
```

2. Run `go generate ./examples/model`

3. Use generated modeltest
Generated file:
```go
type HumanMatcher struct {
	comparator *comparator

	name string
	age  int
}

func NewHumanMatcher(name string, age int) *HumanMatcher {
	return &HumanMatcher{
		name: name,
		age:  age,
	}
}

func (h *HumanMatcher) Matches(arg interface{}) bool {
	h.comparator = &comparator{}

	human, ok := arg.(*model.Human)
	if !ok {
		h.comparator.equal("type", "*model.Human", fmt.Sprintf("%T", arg))
		return h.comparator.matches()
	}

	h.comparator.equal("name", h.name, human.Name())
	h.comparator.equal("age", h.age, human.Age())

	return h.comparator.matches()
}

func (h *HumanMatcher) Got(got interface{}) string {
	return getDiff(h.comparator.got)
}

func (h *HumanMatcher) String() string {
	return getValue(h.comparator.wanted)
}
```

Usge in tests
```go
humanServiceMock = NewHumanServiceMock(gomock.NewController())
...
humanServiceMock.EXPECT().CreateHuman(NewHumanMatcher("joe", 30)).Return(nil)
```
