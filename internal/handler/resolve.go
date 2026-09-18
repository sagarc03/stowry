package handler

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/sagarc03/stowry/internal/service"
	"github.com/sagarc03/stowry/types"
)

const indexDocument = "index.html"

// candidates lists the object paths a request may resolve to, in order. Store
// serves what was asked for, static follows a static host's conventions, and
// spa falls back to the app shell. It runs before the service because neither
// the empty path nor a trailing slash is one the service accepts.
func candidates(mode types.ServerMode, path string) []string {
	if mode == types.ModeStore {
		return []string{path}
	}

	if path == "" {
		return []string{indexDocument}
	}

	// A trailing slash is never an object path, so offering it would end the
	// search on ErrInvalidInput rather than continue it.
	if strings.HasSuffix(path, "/") {
		if mode == types.ModeSPA {
			return []string{path + indexDocument, indexDocument}
		}

		return []string{path + indexDocument}
	}

	if mode == types.ModeSPA {
		return []string{path, indexDocument}
	}

	return []string{path, path + ".html", path + "/" + indexDocument}
}

// resolveGet returns the first candidate that exists and the path it was found
// at. Anything other than a miss stops the search.
func resolveGet(ctx context.Context, svc Service, mode types.ServerMode, path string) (types.MetaData, io.ReadSeekCloser, string, error) {
	var err error

	for _, candidate := range candidates(mode, path) {
		var (
			obj     types.MetaData
			content io.ReadSeekCloser
		)

		obj, content, err = svc.Get(ctx, candidate)
		if !errors.Is(err, service.ErrNotFound) {
			return obj, content, candidate, err
		}
	}

	return types.MetaData{}, nil, path, err
}

// resolveInfo is resolveGet without opening the object.
func resolveInfo(ctx context.Context, svc Service, mode types.ServerMode, path string) (types.MetaData, string, error) {
	var err error

	for _, candidate := range candidates(mode, path) {
		var obj types.MetaData

		obj, err = svc.Info(ctx, candidate)
		if !errors.Is(err, service.ErrNotFound) {
			return obj, candidate, err
		}
	}

	return types.MetaData{}, path, err
}
