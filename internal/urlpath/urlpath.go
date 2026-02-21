package urlpath

import (
	"strings"
)

type ParsedPath struct {
	Prefix string
	Path   string
	Query  string
	Whole  string
}

func Parse(rawPath string) (ParsedPath, error) {
	whole := rawPath

	query := ""
	pathPart := rawPath
	if before, after, ok := strings.Cut(rawPath, "?"); ok {
		query = after
		pathPart = before
	}

	trimmed := strings.TrimPrefix(pathPart, "/")
	prefix, rest, _ := strings.Cut(trimmed, "/")

	return ParsedPath{
		Prefix: prefix,
		Path:   rest,
		Query:  query,
		Whole:  whole,
	}, nil
}
