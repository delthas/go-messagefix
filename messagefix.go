// Package messagefix enables fixing broken email messages, in a best-effort manner,
// so that these messages can be accepted in libraries that strictly follow the email
// specifications.
package messagefix

import (
	"bufio"
	"io"
	"strings"
)

type state int

const (
	stateHeader state = iota
	stateBody
	stateContentType
)

// Reader is an io.Reader that transforms an RFC822 message BODY[] (ie, an EML file content)
// read from an input io.Reader, on-the-fly in a streaming manner.
//
// Reader has several best-effort heuristics to fix broken RFC822 messages slightly so that they
// adhere to the specification. These heuristics are not best-effort and not guaranteed.
//
// Reader may slightly buffer its input io.Reader.
// Reader does not close its input io.Reader.
type Reader struct {
	sc     *bufio.Scanner
	buffer []byte

	boundaries []string

	state state

	bodyIsHeader bool
	contentType  string
}

// NewReader returns a Reader that transforms the passed stream.
//
// Reader does all the buffering it needs, so there is no need to specifically pass a bufio.Reader.
func NewReader(r io.Reader) *Reader {
	return &Reader{
		sc: bufio.NewScanner(r),
	}
}

// Reader follows the general convention of the io.Reader Read method.
//
// See Reader for details.
func (r *Reader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.sc == nil {
		return 0, io.EOF
	}
	if len(r.buffer) == 0 {
		line, err := r.read()
		if err != nil {
			r.sc = nil
			return 0, err
		}
		r.buffer = []byte(line + "\r\n")
	}
	n = copy(p, r.buffer)
	r.buffer = r.buffer[n:]
	return n, nil
}

func parseContentType(content string) (header bool, boundary string) {
	for _, part := range strings.Split(content, ";") {
		part = strings.TrimSpace(part)
		if len(part) == 0 {
			continue
		}
		parts := strings.SplitN(part, "=", 2)
		if len(parts) == 1 {
			switch parts[0] {
			case "message/rfc822", "text/rfc822-headers":
				header = true
			default:
				contentParts := strings.SplitN(parts[0], "/", 2)
				switch contentParts[0] {
				case "multipart":
					header = true
				}
			}
			continue
		}
		if parts[0] == "boundary" {
			boundary = strings.Trim(parts[1], "\"")
			continue
		}
	}
	return
}

func (r *Reader) read() (string, error) {
	if !r.sc.Scan() {
		if err := r.sc.Err(); err != nil {
			return "", err
		}
		// fix: close any remaining open multiparts
		if len(r.boundaries) > 0 {
			if r.state == stateHeader {
				r.state = stateBody
				return "", nil
			}
			line := "--" + r.boundaries[len(r.boundaries)-1] + "--"
			r.boundaries = r.boundaries[:len(r.boundaries)-1]
			return line, nil
		}
		return "", io.EOF
	}
	line := r.sc.Text()
	for i, boundary := range r.boundaries {
		if line == ("--" + boundary + "--") {
			r.boundaries = r.boundaries[:i]
			r.state = stateHeader
			return line, nil
		}
		if line == ("--" + boundary) {
			r.boundaries = r.boundaries[:i+1]
			r.state = stateHeader
			return line, nil
		}
	}
	if r.state == stateContentType {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			r.contentType += strings.Trim(line, " \t")
			return line, nil
		}
		var boundary string
		r.bodyIsHeader, boundary = parseContentType(r.contentType)
		if boundary != "" {
			r.boundaries = append(r.boundaries, boundary)
		}
		r.state = stateHeader
		r.contentType = ""
	}
	if r.state == stateHeader {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "content-type" {
			r.contentType = strings.Trim(parts[1], " \t")
			r.state = stateContentType
			return line, nil
		}
		if line == "" {
			if !r.bodyIsHeader {
				r.state = stateBody
			}
			r.bodyIsHeader = false
			return line, nil
		}
		if !strings.Contains(line, ":") && !(strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
			// fix: indent continuation headers with a space
			line = " " + line
			return line, nil
		}
		return line, nil
	}
	return line, nil
}
