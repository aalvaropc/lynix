package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type CollectionsUC interface {
	List(root string) ([]CollectionItem, error)
	Preview(path string) ([]string, error)
}

func cmdLoadCollections(uc CollectionsUC, root string) tea.Cmd {
	return func() tea.Msg {
		items, _ := uc.List(root)
		return collectionsLoadedMsg(items)
	}
}

func cmdPreviewCollection(uc CollectionsUC, idx int, path string) tea.Cmd {
	return func() tea.Msg {
		names, err := uc.Preview(path)
		return collectionPreviewMsg{Index: idx, Names: names, Err: err}
	}
}

func scanCollections(root string) []CollectionItem {
	dir := filepath.Join(root, "collections")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var items []CollectionItem
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		low := strings.ToLower(name)
		if !(strings.HasSuffix(low, ".yaml") || strings.HasSuffix(low, ".yml")) {
			continue
		}
		items = append(items, CollectionItem{
			Name: name,
			Path: filepath.Join(dir, name),
		})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}
