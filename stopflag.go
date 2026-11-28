// stopflag.go parses the repeatable -stop flag: a stop code, optionally
// followed by a background colour for that stop's screen.
package main

import (
	"fmt"
	"image"
	"strconv"
	"strings"

	"golang.org/x/image/colornames"
)

type stopSpec struct {
	code int
	bg   *image.Uniform // nil means the default background
}

// stopList is a flag.Value collecting one stopSpec per -stop flag.
type stopList []stopSpec

func (l *stopList) String() string {
	return ""
}

// Set parses "code" or "code:colour" and appends it to the list.
func (l *stopList) Set(s string) error {
	code, colorName, hasColor := strings.Cut(s, ":")
	v, err := strconv.Atoi(code)
	if err != nil {
		return err
	}
	spec := stopSpec{code: v}
	if hasColor {
		c, ok := colornames.Map[colorName]
		if !ok {
			return fmt.Errorf("unknown color %q", colorName)
		}
		spec.bg = image.NewUniform(c)
	}
	*l = append(*l, spec)
	return nil
}
