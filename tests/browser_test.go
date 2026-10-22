//go:build browser

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	svghush "github.com/hiteshshimpi-55/svg-hush-go"
)

// TestBrowserRegression runs an optional Chromium check in the standalone SVG
// blob context. Node and Playwright are test-only tools, not library dependencies.
func TestBrowserRegression(t *testing.T) {
	nodePath := os.Getenv("SVG_HUSH_NODE")
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Fatal("browser checks require Node; set SVG_HUSH_NODE to its executable")
		}
	}
	nodePath, err := filepath.Abs(nodePath)
	if err != nil {
		t.Fatal(err)
	}
	positive := `<svg xmlns="http://www.w3.org/2000/svg" onload="parent.__svgHushExecuted++"><script>parent.__svgHushExecuted++</script><rect width="10" height="10"/></svg>`
	inputs := []string{
		positive,
		`<svg xmlns="http://www.w3.org/2000/svg"><foreignObject><body xmlns="http://www.w3.org/1999/xhtml"><img src="x" onerror="parent.__svgHushExecuted++"/></body></foreignObject><rect/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><image href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' onload='parent.__svgHushExecuted++'/%3E"/><rect/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><a id="link"><animate attributeName="href" values="javascript:parent.__svgHushExecuted++" dur="0.001s" fill="freeze"/></a><rect/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><style>svg{background:image-set('https://example.invalid/leak' 1x)}</style><image href="https://example.invalid/leak"/><rect/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><s:svg xmlns:s="http://www.w3.org/2000/svg" xmlns="http://www.w3.org/2000/svg&quot; onload=&quot;parent.__svgHushExecuted++"><rect/></s:svg><rect/></svg>`,
	}
	request := struct {
		Positive string   `json:"positive"`
		Cases    []string `json:"cases"`
	}{Positive: positive}
	for _, input := range inputs {
		output, err := svghush.SanitizeWithOptions([]byte(input), svghush.Options{LocalReferencesOnly: true})
		if err != nil {
			t.Fatalf("SanitizeWithOptions(browser case) error = %v, want nil", err)
		}
		request.Cases = append(request.Cases, string(output))
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, nodePath, "testdata/browser.mjs")
	command.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"TMPDIR=" + os.Getenv("TMPDIR"),
		"SVG_HUSH_PLAYWRIGHT=" + os.Getenv("SVG_HUSH_PLAYWRIGHT"),
		"SVG_HUSH_CHROMIUM=" + os.Getenv("SVG_HUSH_CHROMIUM"),
	}
	command.Stdin = bytes.NewReader(encoded)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("browser regression check failed: %v\n%s", err, output)
	}
	t.Log(string(output))
}
