package activator

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
)

// Enter blocks the conversation loop until the user presses Enter on stdin.
// Each press starts one new "talk" turn. Cooperative with ctx so Ctrl-C
// during a wait unwinds cleanly.
//
// This is the fase 0.2 activator paired with the WhisperCPP STT: the user
// taps Enter, the STT records from the mic, transcribes via whisper.cpp,
// and the orchestrator gets the text. Global-hotkey and wake-word
// activators come in fase 0.3+; they implement the same interface.
type Enter struct {
	reader *bufio.Reader
	prompt string
}

func NewEnter() *Enter {
	return &Enter{
		reader: bufio.NewReader(os.Stdin),
		prompt: "[Enter para hablar] ",
	}
}

func (Enter) Name() string { return "enter" }

func (e *Enter) WaitForActivation(ctx context.Context) error {
	fmt.Print(e.prompt)

	type result struct{ err error }
	ch := make(chan result, 1)
	go func() {
		_, err := e.reader.ReadString('\n')
		if err == io.EOF {
			err = nil
		}
		ch <- result{err: err}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case r := <-ch:
		return r.err
	}
}
