package attrpath

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAttrPath(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		isErr  bool
		expect AttrPath
	}{
		{
			name:   "empty",
			input:  "",
			expect: nil,
		},
		{
			name:  "one value (single char)",
			input: "a",
			expect: AttrPath{
				AttrStepValue("a"),
			},
		},
		{
			name:  "one value",
			input: "aa",
			expect: AttrPath{
				AttrStepValue("aa"),
			},
		},
		{
			name:  "one value (ends with #)",
			input: "a#",
			expect: AttrPath{
				AttrStepValue("a#"),
			},
		},
		{
			name:  "one value (starts with #)",
			input: "#a",
			expect: AttrPath{
				AttrStepValue("#a"),
			},
		},
		{
			name:  "one value (surrounded with #)",
			input: "#a#",
			expect: AttrPath{
				AttrStepValue("#a#"),
			},
		},
		{
			name:  "one splat",
			input: "#",
			expect: AttrPath{
				AttrStepSplat{},
			},
		},
		{
			name:  "consecutive splats",
			input: "##",
			expect: AttrPath{
				AttrStepValue("##"),
			},
		},
		{
			name:  "escaped dot",
			input: `\.`,
			expect: AttrPath{
				AttrStepValue("."),
			},
		},
		{
			name:  "escaped #",
			input: `\#`,
			expect: AttrPath{
				AttrStepValue("#"),
			},
		},
		{
			name:  "escaped \\",
			input: `\\`,
			expect: AttrPath{
				AttrStepValue(`\`),
			},
		},
		{
			name:  "mixed",
			input: `a.#.a\.\#\\b.##`,
			expect: AttrPath{
				AttrStepValue(`a`),
				AttrStepSplat{},
				AttrStepValue(`a.#\b`),
				AttrStepValue(`##`),
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := Path(tt.input)
			if tt.isErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expect, actual)
		})
	}
}

func TestAttrPathString(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "empty",
			input:  "",
			expect: "",
		},
		{
			name:   "one value (single char)",
			input:  "a",
			expect: "a",
		},
		{
			name:   "one value",
			input:  "aa",
			expect: "aa",
		},
		{
			name:   "one value (ends with #)",
			input:  "a#",
			expect: `a\#`,
		},
		{
			name:   "one value (starts with #)",
			input:  "#a",
			expect: `\#a`,
		},
		{
			name:   "one value (surrounded with #)",
			input:  "#a#",
			expect: `\#a\#`,
		},
		{
			name:   "one splat",
			input:  "#",
			expect: "#",
		},
		{
			name:   "consecutive splats",
			input:  "##",
			expect: `\#\#`,
		},
		{
			name:   "escaped dot",
			input:  `\.`,
			expect: `\.`,
		},
		{
			name:   "escaped #",
			input:  `\#`,
			expect: `\#`,
		},
		{
			name:   "escaped \\",
			input:  `\\`,
			expect: `\\`,
		},
		{
			name:   "mixed",
			input:  `a.#.a\.\#\\b.##`,
			expect: `a.#.a\.\#\\b.\#\#`,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := Path(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expect, actual.String())
		})
	}
}
