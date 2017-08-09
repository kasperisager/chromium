package chromium

import (
	"fmt"
	"net"
)

type Flag struct {
	Key   string
	Value interface{}
}

func (flag Flag) String() string {
	switch value := flag.Value.(type) {
	case bool:
		if value {
			return fmt.Sprintf("--%s", flag.Key)
		}
	default:
		return fmt.Sprintf("--%s=%v", flag.Key, value)
	}

	return ""
}

func Address(address net.IP) Flag {
	return Flag{"remote-debugging-address", address}
}

func Port(port uint16) Flag {
	return Flag{"remote-debugging-port", port}
}

func Data(directory string) Flag {
	return Flag{"user-data-dir", directory}
}

func Size(width int, height int) Flag {
	return Flag{"window-size", fmt.Sprintf("%v,%v", width, height)}
}
