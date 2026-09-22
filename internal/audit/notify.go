package audit

import (
	"context"

	"github.com/dmitrymack/go-url-shortener.git/internal/contextkeys"
)

// NotifyFromContext sends an event to publisher (a no-op if publisher is
// nil), tagged with ctx's userID. Shared by the HTTP and gRPC layers so
// "every shorten/follow is logged" is encoded exactly once.
func NotifyFromContext(ctx context.Context, publisher Publisher, action, url string) {
	if publisher == nil {
		return
	}

	userID, _ := contextkeys.UserID(ctx)
	publisher.Notify(NewEvent(action, userID, url))
}
