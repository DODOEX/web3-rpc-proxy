package helpers

import (
	"regexp"
	"strings"

	"github.com/duke-git/lancet/v2/slice"
)

func Concat(strings ...string) string {
	if len(strings) == 2 {
		buf := make([]byte, 0, len(strings[0])+len(strings[1]))
		buf = append(buf, strings[0]...)
		buf = append(buf, strings[1]...)
		return string(buf)
	}

	length := slice.ReduceBy[string, int](strings, 0, func(_ int, item string, agg int) int { return agg + len(item) })
	buf := make([]byte, 0, length)
	for i := 0; i < len(strings); i++ {
		buf = append(buf, strings[i]...)
	}
	return string(buf)
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func ToSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}
