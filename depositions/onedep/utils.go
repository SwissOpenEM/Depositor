package onedep

import (
	"context"
	"net/http"
)

// withBearerToken adds the user's OneDep JWT to the outgoing request.
func WithBearerToken(token string) RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}
}
