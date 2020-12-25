package static

import (
	"strings"
	"testing"

	"github.com/amane3/goreleaser/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestExampleConfig(t *testing.T) {
	_, err := config.LoadReader(strings.NewReader(ExampleConfig))
	require.NoError(t, err)
}
