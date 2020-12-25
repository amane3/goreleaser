package middleware

import (
	"fmt"
	"testing"

	"github.com/amane3/goreleaser/internal/pipe"
	"github.com/stretchr/testify/require"
)

func TestError(t *testing.T) {
	t.Run("no errors", func(t *testing.T) {
		require.NoError(t, ErrHandler(mockAction(nil))(ctx))
	})

	t.Run("pipe skipped", func(t *testing.T) {
		require.NoError(t, ErrHandler(mockAction(pipe.ErrSkipValidateEnabled))(ctx))
	})

	t.Run("some err", func(t *testing.T) {
		require.Error(t, ErrHandler(mockAction(fmt.Errorf("pipe errored")))(ctx))
	})
}
