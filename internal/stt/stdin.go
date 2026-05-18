package stt

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// Stdin is a placeholder STT used during early development: the user types
// what they would have said. Replaced by whisper.cpp in fase 0.2.
type Stdin struct {
	reader *bufio.Reader
}

func NewStdin() *Stdin {
	return &Stdin{reader: bufio.NewReader(os.Stdin)}
}

func (s *Stdin) Name() string { return "stdin" }

func (s *Stdin) Listen(ctx context.Context) (string, error) {
	fmt.Print("you> ")

	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		line, err := s.reader.ReadString('\n')
		ch <- result{line: strings.TrimSpace(line), err: err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-ch:
		if r.err != nil && r.err != io.EOF {
			return "", r.err
		}
		return r.line, nil
	}
}
