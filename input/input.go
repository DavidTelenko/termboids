package input

import (
	"os"
	"sync"

	"golang.org/x/term"
)

// Handler manages keyboard input in a non-blocking manner
type Handler struct {
	oldState *term.State
	mu       sync.Mutex
	events   chan rune
	stop     chan struct{}
}

// NewHandler creates and initializes a new input handler
func NewHandler() (*Handler, error) {
	// Save the current terminal state
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}

	h := &Handler{
		oldState: oldState,
		events:   make(chan rune, 100),
		stop:     make(chan struct{}),
	}

	// Start listening for input in a goroutine
	go h.listen()

	return h, nil
}

// listen continuously reads from stdin and sends keys to the events channel
func (h *Handler) listen() {
	buf := make([]byte, 3)
	for {
		select {
		case <-h.stop:
			return
		default:
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				continue
			}

			// Handle single byte characters
			if n == 1 {
				h.events <- rune(buf[0])
			}
			// Could extend this to handle multi-byte sequences (arrow keys, etc.)
		}
	}
}

// Poll returns the next key press if available, or 0 if no key is pending
func (h *Handler) Poll() rune {
	select {
	case key := <-h.events:
		return key
	default:
		return 0
	}
}

// Close restores the terminal state and stops the input handler
func (h *Handler) Close() error {
	close(h.stop)
	if h.oldState != nil {
		return term.Restore(int(os.Stdin.Fd()), h.oldState)
	}
	return nil
}
