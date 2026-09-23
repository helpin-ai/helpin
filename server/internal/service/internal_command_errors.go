package service

import "fmt"

// CommandErrorKind classifies a command failure the caller can act on.
type CommandErrorKind string

const (
	// CommandErrorNotFound means a referenced record does not exist in the
	// caller's workspace or is not visible to the caller.
	CommandErrorNotFound CommandErrorKind = "not_found"
	// CommandErrorInvalidInput means the input is well-formed but
	// semantically invalid, such as a missing required value or a state that
	// does not belong to the task's workflow.
	CommandErrorInvalidInput CommandErrorKind = "invalid_input"
	// CommandErrorForbidden means the caller may not act on the record.
	CommandErrorForbidden CommandErrorKind = "forbidden"
)

// CommandError is a command failure whose Message was written for the caller.
// Messages must never contain SQL, internal paths, or data from another
// workspace; public boundaries forward them verbatim.
type CommandError struct {
	Kind    CommandErrorKind
	Message string
}

// Error implements error. It returns Message unchanged so existing callers
// that compare error text keep working.
func (e *CommandError) Error() string { return e.Message }

// errCommandNotFound reports that an entity (for example "task") is missing
// or not visible, without revealing which.
func errCommandNotFound(entity string) error {
	return &CommandError{Kind: CommandErrorNotFound, Message: entity + " not found"}
}

// errCommandInput reports caller-fixable invalid input. Format arguments must
// be caller-supplied values or constants, never data loaded from storage.
func errCommandInput(format string, args ...any) error {
	return &CommandError{Kind: CommandErrorInvalidInput, Message: fmt.Sprintf(format, args...)}
}
