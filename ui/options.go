package ui

import "github.com/peco/peco/internal/option"

type Option = option.Interface

const (
	optkeyLineCache    = "line-cache"
	optkeyRunningQuery = "running-query"
)

func WithLineCache(b bool) Option {
	return option.New(optkeyLineCache, b)
}

func WithRunningQuery(b bool) Option {
	return option.New(optkeyRunningQuery, b)
}

