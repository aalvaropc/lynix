package tui

import "time"

type RunnerEvent struct {
	RequestName     string
	Index           int
	Total           int
	Status          int
	Latency         time.Duration
	Pass            bool
	Explain         string
	AssertionsText  string
	ResponseSnippet string
}

type runnerEventMsg RunnerEvent
type runnerDoneMsg struct {
	Err error
}
