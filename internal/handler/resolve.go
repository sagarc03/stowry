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

// candidates lists the object paths a request may resolve to, in the order to
// try them.
//
// Store mode serves exactly what was asked for. Static mode follows the
// convention a static host uses: a directory URL means its index, and a bare
// name may be a page. SPA mode falls back to the app shell so client-side
// routing can take over.
//
// Neither the empty path nor a trailing slash is a path the service accepts,
// which is why this runs before it rather than inside it.
func candidates(mode types.ServerMode, path string) []string {
	if mode == types.ModeStore {
		return []string{path}
	}

	if path == "" {
		return []string{indexDocument}
	}

	// A trailing slash names a directory, which is never an object path, so it
	// must not be offered as a candidate: the service rejects it outright and
	// that would end the search rather than continue it.
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

// resolveGet returns the first candidate that exists, along with the path it
// was found at, which names the response for content type sniffing and ranges.
// Anything other than a miss stops the search.
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
