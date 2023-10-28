package modeltest

import (
	"fmt"
	"strings"
)

type comparator struct {
	wanted []attribute
	got    []attribute
}

func (c *comparator) equal(key string, want, got any) {
	if want != got {
		c.wanted = append(c.wanted, attribute{name: key, value: want})
		c.got = append(c.got, attribute{name: key, value: got})
	}
}

func (c *comparator) matches() bool {
	return len(c.wanted) != 0
}

type attribute struct {
	name  string
	value any
}

func getDiff(fields []attribute) string {
	diffs := make([]string, len(fields))
	for i := range fields {
		diffs[i] = fmt.Sprintf("%s=%s", fields[i].name, getValue(fields[i].value))
	}

	return strings.Join(diffs, " ")
}

func getValue(value interface{}) string {
	switch value.(type) {
	case int, int32, bool:
		return fmt.Sprintf("\"%v\"", value)
	}
	return fmt.Sprintf("%q", value)
}
