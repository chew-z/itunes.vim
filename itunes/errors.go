package itunes

import "fmt"

type ITunesError struct {
	Op      string    // Operation that failed
	Kind    ErrorKind // Type of error
	Err     error     // Underlying error
	Context map[string]interface{}
}

type ErrorKind int

const (
	ErrDatabase ErrorKind = iota
	ErrAppleMusic
	ErrJXAScript
	ErrNetwork
	ErrPermission
	ErrNoTracksFound
)

func (e *ITunesError) Error() string {
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}
