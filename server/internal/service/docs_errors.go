package service

import "errors"

// Sentinel errors returned by the docs service layer. Handlers should use
// errors.Is to map these to HTTP status codes via DocsErrorStatus.
//
// These errors are deliberately narrow and domain-specific so the
// transport layer can translate each case into a user-friendly response
// without parsing error strings.
var (
	// ErrDocsCollectionNotFound is returned when a referenced collection
	// does not exist or has been soft-deleted.
	ErrDocsCollectionNotFound = errors.New("collection not found")

	// ErrDocsCollectionParentNotFound is returned when a create or update
	// request references a parent collection that does not exist.
	ErrDocsCollectionParentNotFound = errors.New("parent collection not found")

	// ErrDocsCollectionParentDifferentSpace is returned when a parent
	// collection exists but belongs to a different space than the
	// requested collection.
	ErrDocsCollectionParentDifferentSpace = errors.New("parent collection belongs to a different space")

	// ErrDocsCollectionSelfParent is returned when an update request
	// attempts to set a collection as its own parent.
	ErrDocsCollectionSelfParent = errors.New("collection cannot be its own parent")

	// ErrDocsCollectionCycle is returned when a reparent request would
	// create a cycle by placing the collection under one of its
	// descendants.
	ErrDocsCollectionCycle = errors.New("cannot move collection under one of its descendants")

	// ErrDocsCollectionDepthExceeded is returned when a create or update
	// request would push a collection past the maximum supported depth.
	ErrDocsCollectionDepthExceeded = errors.New("collection depth exceeds maximum")

	// ErrDocsSpaceNotFound is returned when a referenced space does not
	// exist or belongs to a different workspace.
	ErrDocsSpaceNotFound = errors.New("space not found")

	// ErrDocsCrossWorkspace is returned when a request targets a space
	// that does not belong to the calling workspace.
	ErrDocsCrossWorkspace = errors.New("space does not belong to this workspace")

	// ErrDocsCollectionNameRequired is returned when a create request
	// omits the required name field.
	ErrDocsCollectionNameRequired = errors.New("collection name is required")

	// ErrDocsStaleBlockRevision is returned when a block patch is based on
	// an older revision than the current stored block.
	ErrDocsStaleBlockRevision = errors.New("block revision is stale")

	// ErrDocsDocumentLocked is returned when a mutation targets a locked
	// document.
	ErrDocsDocumentLocked = errors.New("document is locked and cannot be modified")
)
