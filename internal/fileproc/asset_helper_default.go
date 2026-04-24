package fileproc

import (
	"context"
	"regexp"
)

type DefaultHandler struct {
	regexps []*regexp.Regexp
}

func NewDefaultHandler(exps []string) DefaultHandler {
	regexps := make([]*regexp.Regexp, len(exps))
	for i, s := range exps {
		regexps[i] = regexp.MustCompile(regexp.QuoteMeta(s))
	}
	return DefaultHandler{regexps}
}

func (h DefaultHandler) Supports(mime string) bool {
	for _, prefix := range h.regexps {
		if prefix.MatchString(mime) {
			return true
		}
	}
	return false
}

func (h DefaultHandler) ExtractMeta(ctx context.Context, filePath string) (FileMeta, error) {
	return FileMeta{}, nil
}
