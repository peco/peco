package peco

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	CaseSensitiveMatch = iota
	IgnoreCaseMatch
)

type CaseSensitiveMatcher struct {
}

func (m *CaseSensitiveMatcher) QueryToRegexps(query string) ([]*regexp.Regexp, error) {
	queries := strings.Split(strings.TrimSpace(query), " ")
	regexps := make([]*regexp.Regexp, 0)

	for _, q := range queries {
		reTxt := fmt.Sprintf("%s", regexp.QuoteMeta(q))
		re, err := regexp.Compile(reTxt)
		if err != nil {
			return nil, err
		}
		regexps = append(regexps, re)
	}

	return regexps, nil
}

func (m *CaseSensitiveMatcher) String() string {
	return "CaseSentive"
}

type IgnoreCaseMatcher struct {
}

func (m *IgnoreCaseMatcher) QueryToRegexps(query string) ([]*regexp.Regexp, error) {
	queries := strings.Split(strings.TrimSpace(query), " ")
	regexps := make([]*regexp.Regexp, 0)

	for _, q := range queries {
		reTxt := fmt.Sprintf("(?i)%s", regexp.QuoteMeta(q))
		re, err := regexp.Compile(reTxt)
		if err != nil {
			return nil, err
		}
		regexps = append(regexps, re)
	}

	return regexps, nil
}

func (m *IgnoreCaseMatcher) String() string {
	return "IgnoreCase"
}

