package ports

// WorkspaceLocator finds a Lynix workspace root starting from an arbitrary directory.
type WorkspaceLocator interface {
	FindRoot(startDir string) (string, error)
}
