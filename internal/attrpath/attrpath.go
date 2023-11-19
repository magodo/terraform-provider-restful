package attrpath

import (
	"fmt"
	"strings"
	"text/scanner"
)

type AttrPath []AttrStep

func (p AttrPath) String() string {
	var ol []string
	for _, step := range p {
		switch step := step.(type) {
		case AttrStepValue:
			var val string
			for _, c := range step {
				switch c {
				case '.', '#', '\\':
					val += `\` + string(c)
				default:
					val += string(c)
				}
			}
			ol = append(ol, val)
		case AttrStepSplat:
			ol = append(ol, "#")
		}
	}
	return strings.Join(ol, ".")
}

type AttrStep interface {
	isAttrStep()
}

type AttrStepValue string

func (AttrStepValue) isAttrStep() {}

type AttrStepSplat struct{}

func (AttrStepSplat) isAttrStep() {}

func ParseAttrPath(input string) (AttrPath, error) {
	var gerr error
	s := scanner.Scanner{
		Error: func(s *scanner.Scanner, msg string) {
			gerr = fmt.Errorf("%s: %s", s.Pos().String(), msg)
		},
	}
	s.Init(strings.NewReader(input))
	var escape bool
	var path AttrPath
	var value AttrStepValue
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		if gerr != nil {
			return nil, gerr
		}

		tk := s.TokenText()

		if escape {
			escape = false
			value += AttrStepValue(tk)
			continue
		}

		switch tk {
		case "\\":
			escape = true
			continue
		case ".":
			if s.Peek() == '.' {
				return nil, fmt.Errorf("%s: consecutive '.' is not allowed", s.Pos().String())
			}
			path = append(path, value)
			value = ""
			continue
		case "#":
			// If there is prepending value for this step, just append the "#", as part of the value
			if value != "" {
				value += "#"
				continue
			}

			if peek := s.Peek(); peek == '.' || peek == scanner.EOF {
				path = append(path, AttrStepSplat{})
				s.Scan()
				continue
			}

			// If the next tok is not a '.', then regard "#" as a regular value
			value += "#"
			continue
		default:
			value += AttrStepValue(tk)
			continue
		}
	}
	if value != "" {
		path = append(path, value)
	}
	return path, nil
}
