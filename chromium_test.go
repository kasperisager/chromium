package chromium_test

import (
	"fmt"

	chromium "github.com/kasperisager/chromium"
)

func ExampleNew() {
	chromium.New("google-chrome", chromium.Port(9222))
}

func ExampleNew_flags() {
	chromium.New("google-chrome", chromium.Port(9222), chromium.WindowSize(1920, 1080))
}

func ExampleNew_ephemeral() {
	browser := chromium.New("google-chrome")
	port, err := browser.Start()

	if err != nil {
		// Handle err
	}

	fmt.Println(port)
}
