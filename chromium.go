package chromium

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	watcher "gopkg.in/radovskyb/watcher.v1"
)

var (
	ErrProcessRunning    = errors.New("chromium: Process is already running")
	ErrProcessNotRunning = errors.New("chromium: Process is not running")
	ErrNoPortAssigned    = errors.New("chromium: No port assigned to process")
)

var (
	// https://support.google.com/chrome/a/answer/6271282
	errRegex = regexp.MustCompile(`\[.*:(.+)\((\d+)\)\]\s+(.+)`)
)

// The Chromium interface describes a Chromium browser process.
type Chromium interface {
	// Get the path to the Chromium binary.
	Path() string

	// Get the address of the remote debugging endpoint.
	Addr() net.IP

	// Get the port of the remote debugging endpoint. In the event that an
	// ephemeral port has been requested and the port is attempted read before
	// the process has been started, an error will be returned.
	Port() (uint16, error)

	// Get the user data directory of the Chromium process.
	Data() (string, error)

	// Start the Chromium process without waiting for it to finish. Start returns
	// only when the remote debugging endpoint is ready to serve clients. Start is
	// idempotent and invoking it on an already running process has no effect.
	Start() error

	// Stop the Chromium process. Stop is idempotent and invoking it on an already
	// stopped process has no effect.
	Stop() error

	// Wait for the Chromium process to finish. Wait is idempotent and invoking
	// it on an already stopped process has no effect.
	Wait() error

	// Start the Chromium process and wait for it to finish. Run is idempotent
	// and invoking it on an already running process has no effect.
	Run() error

	// Read-only channel of errors emitted by the Chromium process.
	Errors() <-chan error
}

type chromium struct {
	path  string
	addr  net.IP
	port  uint16
	data  string
	cmd   *exec.Cmd
	flags []string
	errs  chan error
}

type Error struct {
	file string
	line int
	msg  string
}

// New returns a new Chromium process using the specified path, address, port,
// and flags. A complete list of available Chromium flags can be found at:
// https://peter.sh/experiments/chromium-command-line-switches/
func New(path string, addr net.IP, port uint16, flags ...string) Chromium {
	return &chromium{
		path:  path,
		addr:  addr,
		port:  port,
		flags: flags,
		errs:  make(chan error, 1),
	}
}

func (chromium *chromium) Path() string {
	return chromium.path
}

func (chromium *chromium) Addr() net.IP {
	return chromium.addr
}

func (chromium *chromium) Port() (uint16, error) {
	if chromium.port == 0 {
		return 0, ErrNoPortAssigned
	}

	return chromium.port, nil
}

func (chromium *chromium) Data() (string, error) {
	if chromium.cmd == nil {
		return "", ErrProcessNotRunning
	}

	return chromium.data, nil
}

func (chromium *chromium) Errors() <-chan error {
	return chromium.errs
}

func (chromium *chromium) Start() error {
	if chromium.cmd != nil {
		return ErrProcessRunning
	}

	data, err := ioutil.TempDir("", "chromium-")

	if err != nil {
		return err
	}

	chromium.data = data

	flags := append(chromium.flags,
		"--headless",    // Run Chromium in headless mode
		"--disable-gpu", // Disable GPU support as it does not work in headless mode
		"--no-sandbox",  // Disable sandboxing as it does not work in containers

		fmt.Sprintf("--user-data-dir=%v", chromium.data),
		fmt.Sprintf("--remote-debugging-address=%v", chromium.addr),
		fmt.Sprintf("--remote-debugging-port=%v", chromium.port),
	)

	chromium.cmd = exec.Command(chromium.path, flags...)

	stderr, err := chromium.cmd.StderrPipe()

	if err != nil {
		return err
	}

	go chromium.Scan(stderr)

	poller := watcher.New()

	defer poller.Close()

	if err := poller.Add(chromium.data); err != nil {
		return err
	}

	go poller.Start(20 * time.Millisecond)

	if err := chromium.cmd.Start(); err != nil {
		return err
	}

	active := false

	for !active {
		select {
		case event := <-poller.Event:
			if event.Name() != "DevToolsActivePort" {
				continue
			}

			file, err := ioutil.ReadFile(event.Path)

			if err != nil {
				return err
			}

			port, err := strconv.ParseUint(string(file), 10, 16)

			if err != nil {
				return err
			}

			chromium.port = uint16(port)

			active = true
		case err := <-poller.Error:
			return err
		case err := <-chromium.errs:
			return err
		}
	}

	return nil
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

func (chromium *chromium) Run() error {
	if err := chromium.Start(); err != nil {
		return err
	}

	return chromium.Wait()
}

func (chromium *chromium) Cleanup() {
	if chromium.data != "" {
		os.RemoveAll(chromium.data)
	}

	chromium.cmd = nil
	chromium.data = ""
}

func (chromium *chromium) Scan(pipe io.ReadCloser) {
	defer pipe.Close()

	scanner := bufio.NewScanner(pipe)

	for scanner.Scan() {
		parts := errRegex.FindStringSubmatch(scanner.Text())

		line, _ := strconv.ParseInt(parts[2], 10, 32)

		chromium.errs <- &Error{
			file: parts[1],
			line: int(line),
			msg:  parts[3],
		}
	}

	if err := scanner.Err(); err != nil {
		chromium.errs <- err
	}
}

func (error *Error) Error() string {
	return "chromium: " + error.msg
}

func (error *Error) File() string {
	return error.file
}

func (error *Error) Line() int {
	return error.line
}
