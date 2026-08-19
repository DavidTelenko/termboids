package input

import (
	"os"
	"sync"

	"golang.org/x/term"
)

// MouseEvent represents a mouse click event
type MouseEvent struct {
	X      int
	Y      int
	Button int // 0 = left, 1 = middle, 2 = right
}

// Handler manages keyboard and mouse input in a non-blocking manner
type Handler struct {
	oldState    *term.State
	mu          sync.Mutex
	events      chan rune
	mouseEvents chan MouseEvent
	stop        chan struct{}
}

// NewHandler creates and initializes a new input handler
func NewHandler() (*Handler, error) {
	// Save the current terminal state
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}

	h := &Handler{
		oldState:    oldState,
		events:      make(chan rune, 100),
		mouseEvents: make(chan MouseEvent, 100),
		stop:        make(chan struct{}),
	}

	// Start listening for input in a goroutine
	go h.listen()

	return h, nil
}

// listen continuously reads from stdin and sends keys to the events channel
func (h *Handler) listen() {
	buf := make([]byte, 16) // Larger buffer for mouse sequences
	for {
		select {
		case <-h.stop:
			return
		default:
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				continue
			}

			// Check for mouse event sequence: ESC [ < button ; x ; y M (press) or m (release)
			// SGR extended mouse format: \x1b[<0;x;yM for left click press
			if n >= 6 && buf[0] == 0x1b && buf[1] == '[' && buf[2] == '<' {
				// Parse mouse event
				var button, x, y int
				var eventType byte
				
				// Find semicolons and parse coordinates
				parsed := 0
				numStart := 3
				for i := 3; i < n; i++ {
					if buf[i] == ';' || buf[i] == 'M' || buf[i] == 'm' {
						if buf[i] == 'M' || buf[i] == 'm' {
							eventType = buf[i]
							// Parse last number (y coordinate)
							if parsed == 2 {
								y = 0
								for j := numStart; j < i; j++ {
									if buf[j] >= '0' && buf[j] <= '9' {
										y = y*10 + int(buf[j]-'0')
									}
								}
							}
							break
						}
						
						// Parse current number
						num := 0
						for j := numStart; j < i; j++ {
							if buf[j] >= '0' && buf[j] <= '9' {
								num = num*10 + int(buf[j]-'0')
							}
						}
						
						if parsed == 0 {
							button = num
						} else if parsed == 1 {
							x = num
						}
						parsed++
						numStart = i + 1
					}
				}
				
				// Handle left click (button 0) and right click (button 2), both on press (eventType 'M')
				if (button == 0 || button == 2) && eventType == 'M' {
					h.mouseEvents <- MouseEvent{X: x - 1, Y: y - 1, Button: button} // Convert to 0-based
				}
			} else if n == 1 {
				// Handle single byte characters
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

// PollMouse returns the next mouse event if available
func (h *Handler) PollMouse() *MouseEvent {
	select {
	case event := <-h.mouseEvents:
		return &event
	default:
		return nil
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
