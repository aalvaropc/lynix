package tui

type CollectionItem struct {
	Name         string
	Path         string
	RequestNames []string
	ParseErr     error
}

type collectionsLoadedMsg []CollectionItem
type collectionPreviewMsg struct {
	Index int
	Names []string
	Err   error
}
