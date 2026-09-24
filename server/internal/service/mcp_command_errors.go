package service

import (
	"context"
	"errors"
	"strings"
	"unicode"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Typed public error codes for command-backed MCP tools.
const (
	// MCPErrorCodeNotFound means a referenced record is missing or not visible
	// in the connected workspace.
	MCPErrorCodeNotFound = "NOT_FOUND"
	// MCPErrorCodeInvalidInput means schema-valid arguments that the command
	// rejected, such as a state from another workflow.
	MCPErrorCodeInvalidInput = "INVALID_INPUT"
	// MCPErrorCodeVersionConflict means the document changed after the caller
	// read it.
	MCPErrorCodeVersionConflict = "VERSION_CONFLICT"
	// MCPErrorCodeForbidden means the caller may not act on the record.
	MCPErrorCodeForbidden = "FORBIDDEN"
)

const (
	_mcpVersionConflictMessage = "The document changed; read it again and retry with the new version."
	_mcpNotFoundMessage        = "The requested Helpin record was not found in the connected workspace."
	_mcpForbiddenMessage       = "You do not have access to this Helpin record."
	_mcpInvalidContentMessage  = "The resulting document content is not valid. Check the content and retry."
)

// mcpCommandError translates a command failure into a public MCP error.
// Only whitelisted error types are made specific; their messages are either
// fixed here or were written for the caller (CommandError). Everything else
// is returned unchanged and reaches clients as the generic tool failure, so
// raw internal errors are never forwarded.
func mcpCommandError(err error) error {
	if err == nil {
		return nil
	}
	var toolErr *MCPToolError
	if errors.As(err, &toolErr) || isMCPBoundaryError(err) {
		return err
	}
	var commandErr *CommandError
	var forbidden *model.ErrForbidden
	switch {
	case errors.Is(err, ErrDocsContentConflict), errors.Is(err, ErrDocsStaleBlockRevision):
		return newMCPToolError(MCPErrorCodeVersionConflict, _mcpVersionConflictMessage)
	case errors.Is(err, ErrDocsDocumentLocked):
		return newMCPToolError(MCPErrorCodeDocumentLocked, "The document is locked. Unlock it in Helpin first.")
	case errors.Is(err, ErrDocsInvalidContent):
		return newMCPToolError(MCPErrorCodeInvalidInput, _mcpInvalidContentMessage)
	case errors.As(err, &commandErr):
		return newMCPToolError(mcpCommandErrorCode(commandErr.Kind), mcpSentence(commandErr.Message))
	case errors.As(err, &forbidden):
		return newMCPToolError(MCPErrorCodeForbidden, _mcpForbiddenMessage)
	case errors.Is(err, gorm.ErrRecordNotFound):
		return newMCPToolError(MCPErrorCodeNotFound, _mcpNotFoundMessage)
	default:
		return err
	}
}

// isMCPBoundaryError reports errors that publicToolError already explains.
func isMCPBoundaryError(err error) bool {
	for _, known := range []error{
		ErrMCPUnauthorized, ErrMCPForbidden, ErrMCPDisabled, ErrMCPNotFound, ErrMCPConflict,
		ErrMCPInvalidArguments, ErrMCPRateLimited, context.DeadlineExceeded,
	} {
		if errors.Is(err, known) {
			return true
		}
	}
	return false
}

func mcpCommandErrorCode(kind CommandErrorKind) string {
	switch kind {
	case CommandErrorNotFound:
		return MCPErrorCodeNotFound
	case CommandErrorForbidden:
		return MCPErrorCodeForbidden
	default:
		return MCPErrorCodeInvalidInput
	}
}

// mcpSentence capitalizes a command message and ends it with a period.
func mcpSentence(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "The request could not be completed with these arguments."
	}
	runes := []rune(message)
	runes[0] = unicode.ToUpper(runes[0])
	message = string(runes)
	if !strings.HasSuffix(message, ".") {
		message += "."
	}
	return message
}
