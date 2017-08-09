package chromium

import (
	"bufio"
	"io"
	"strconv"
	"regexp"
)

var (
	// https://support.google.com/chrome/a/answer/6271282
	errFormat = regexp.MustCompile(`\[.*:(.+)\((\d+)\)\]\s+(.+)`)
)

type Error struct {
	file string
	line int
	msg  string
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

func (error *Error) Message() string {
	return error.msg
}

func Scan(in io.Reader, out chan<- error) {
	scanner := bufio.NewScanner(in)

	for scanner.Scan() {
		parts := errFormat.FindStringSubmatch(scanner.Text())

		line, _ := strconv.ParseInt(parts[2], 10, 32)

		out <- &Error{
			file: parts[1],
			line: int(line),
			msg:  parts[3],
		}
	}

	if err := scanner.Err(); err != nil {
		out <- err
	}
}
