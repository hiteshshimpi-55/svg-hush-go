package svghush_test

import (
	"fmt"
	"log"

	svghush "github.com/hiteshshimpi-55/svg-hush-go"
)

func ExampleSanitize() {
	input := []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"><script>alert(2)</script><rect width="10"/></svg>`)
	output, err := svghush.Sanitize(input)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(output))
	// Output: <svg xmlns="http://www.w3.org/2000/svg"><rect width="10"></rect></svg>
}

func ExampleSanitizeWithOptions() {
	input := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><use href="#icon"/><image href="https://example.com/logo.png"/></svg>`)
	output, err := svghush.SanitizeWithOptions(input, svghush.Options{LocalReferencesOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(output))
	// Output: <svg xmlns="http://www.w3.org/2000/svg"><use href="#icon"></use><image></image></svg>
}
