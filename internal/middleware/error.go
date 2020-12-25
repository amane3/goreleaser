package middleware

import (
	"github.com/amane3/goreleaser/internal/pipe"
	"github.com/amane3/goreleaser/pkg/context"
	"github.com/apex/log"
)

// ErrHandler handles an action error, ignoring and logging pipe skipped
// errors.
func ErrHandler(action Action) Action {
	return func(ctx *context.Context) error {
		var err = action(ctx)
		if err == nil {
			return nil
		}
		if pipe.IsSkip(err) {
			log.WithError(err).Warn("pipe skipped")
			return nil
		}
		return err
	}
}
