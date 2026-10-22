package tests

import (
	"bytes"
	"testing"

	svghush "github.com/hiteshshimpi-55/svg-hush-go"
)

func FuzzSanitize(f *testing.F) {
	for _, input := range []string{
		`<svg xmlns="http://www.w3.org/2000/svg"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"><script>x</script></svg>`,
		`<s:svg xmlns:s="http://www.w3.org/2000/svg" xmlns="wrong"><rect/></s:svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><style>g{fill:url(//evil.invalid/a)}</style></svg>`,
		`<!DOCTYPE svg><svg xmlns="http://www.w3.org/2000/svg"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><animate attributeName="href" values="javascript:alert(1)"/></svg>`,
	} {
		f.Add([]byte(input))
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		output, err := svghush.Sanitize(input)
		if err != nil {
			if output != nil {
				t.Fatalf("svghush.Sanitize(%q) returned output with error %v", input, err)
			}
			return
		}
		assertSafeOutput(t, output)
		second, err := svghush.SanitizeWithOptions(output, svghush.Options{MaxInputBytes: svghush.DefaultMaxOutputBytes})
		if err != nil || !bytes.Equal(output, second) {
			t.Fatalf("svghush.Sanitize output is not idempotent: first %q, second %q, error %v", output, second, err)
		}
	})
}
