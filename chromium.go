package chromium

import (
	"errors"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"

	watcher "gopkg.in/radovskyb/watcher.v1"
)

var (
	ErrProcessRunning    = errors.New("chromium: Process is already running")
	ErrProcessNotRunning = errors.New("chromium: Process is not running")
	ErrNoPortAssigned    = errors.New("chromium: No port assigned to process")
)

// The Chromium interface describes a Chromium browser process.
type Chromium interface {
	// Start the Chromium process without waiting for it to finish. Start returns
	// only when the remote debugging endpoint is ready to serve clients. Start is
	// idempotent and invoking it on an already running process has no effect.
	Start() (uint16, error)

	// Stop the Chromium process. Stop is idempotent and invoking it on an already
	// stopped process has no effect.
	Stop() error

	// Wait for the Chromium process to finish. Wait is idempotent and invoking
	// it on an already stopped process has no effect.
	Wait() error

	// Read-only channel of errors emitted by the Chromium process.
	Errors() <-chan error
}

type chromium struct {
	path  string
	data  string
	flags []Flag
	errs  chan error
	cmd   *exec.Cmd
}

// New returns a new Chromium process using the flags. A complete list of
// available Chromium flags can be found at:
// https://peter.sh/experiments/chromium-command-line-switches/
func New(path string, flags ...Flag) Chromium {
	return &chromium{
		path:  path,
		flags: flags,
		errs:  make(chan error, 1),
	}
}

func (chromium *chromium) Flag(key string) (interface{}, bool) {
	for _, flag := range chromium.flags {
		if flag.Key == key {
			return flag.Value, true
		}
	}

	return nil, false
}

func (chromium *chromium) Errors() <-chan error {
	return chromium.errs
}

func (chromium *chromium) Start() (uint16, error) {
	if chromium.cmd != nil {
		return 0, ErrProcessRunning
	}

	chromium.flags = append(chromium.flags,
		Flag{"headless", true},
		Flag{"disable-gpu", true},
		Flag{"no-sandbox", true},
	)

	if data, has := chromium.Flag("user-data-dir"); has {
		chromium.data = data.(string)
	} else {
		data, err := ioutil.TempDir("", "chromium-")

		if err != nil {
			return 0, err
		}

		chromium.flags = append(chromium.flags, Flag{"user-data-dir", data})
		chromium.data = data
	}

	if _, has := chromium.Flag("remote-debugging-address"); !has {
		chromium.flags = append(chromium.flags, Flag{"remote-debugging-address", net.IPv4(127, 0, 0, 1)})
	}

	if _, has := chromium.Flag("remote-debugging-port"); !has {
		chromium.flags = append(chromium.flags, Flag{"remote-debugging-port", 0})
	}

	flags := make([]string, len(chromium.flags))

	for i, flag := range chromium.flags {
		flags[i] = flag.String()
	}

	chromium.cmd = exec.Command(chromium.path, flags...)

	stderr, err := chromium.cmd.StderrPipe()

	if err != nil {
		return 0, err
	}

	go Scan(stderr, chromium.errs)

	poller := watcher.New()

	defer poller.Close()

	if err := poller.Add(chromium.data); err != nil {
		return 0, err
	}

	go poller.Start(20 * time.Millisecond)

	if err := chromium.cmd.Start(); err != nil {
		return 0, err
	}

	for {
		select {
		case event := <-poller.Event:
			if event.Name() != "DevToolsActivePort" {
				continue
			}

			file, err := ioutil.ReadFile(event.Path)

			if err != nil {
				return 0, err
			}

			port, err := strconv.ParseUint(string(file), 10, 16)

			if err != nil {
				return 0, err
			}

			return uint16(port), nil
		case err := <-poller.Error:
			return 0, err
		case err := <-chromium.errs:
			return 0, err
		}
	}
}

func (chromium *chromium) Stop() error {
	if chromium.cmd == nil {
		return ErrProcessNotRunning
	}

	defer chromium.Cleanup()

	if err := chromium.cmd.Process.Kill(); err != nil {
		return err
	}

	return nil
}

func (chromium *chromium) Wait() error {
	if chromium.cmd == nil {
		return ErrProcessNotRunning
	}

	defer chromium.Cleanup()

	if err := chromium.cmd.Wait(); err != nil {
		return err
	}

	return nil
}

func (chromium *chromium) Cleanup() {
	if chromium.data != "" {
		os.RemoveAll(chromium.data)
	}

	chromium.cmd = nil
	chromium.data = ""
}
