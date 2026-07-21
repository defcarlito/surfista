package app

import (
	"surfista/internal/screens/search"
	"surfista/internal/surf"
)

type screen int

const (
	homeScreen screen = iota
	searchScreen
)

type Model struct {
	current screen
	search  search.Model
	tracked []surf.Spot
	loadErr error
}

func New(searcher surf.SpotSearcher, tracker search.Tracker, tracked []surf.Spot, loadErr error) Model {
	return Model{
		current: homeScreen,
		search:  search.New(searcher, tracker),
		tracked: tracked,
		loadErr: loadErr,
	}
}
