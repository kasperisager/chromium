package chromium_test

import (
	"fmt"
	"net"

	chromium "github.com/kasperisager/chromium"
)

func ExampleNew() {
	chromium.New("google-chrome", net.IPv4(127, 0, 0, 1), 9222)
}

func ExampleNew_flags() {
	chromium.New("google-chrome", net.IPv4(127, 0, 0, 1), 9222,
		"--disable-extensions",
		"--window-size=1920,1080",
	)
}

func ExampleNew_ephemeral() {
	browser := chromium.New("google-chrome", net.IPv4(127, 0, 0, 1), 0)

	if err := browser.Start(); err != nil {
		// Handle err
	}

	port, _ := browser.Port()

	fmt.Println(port)
}
