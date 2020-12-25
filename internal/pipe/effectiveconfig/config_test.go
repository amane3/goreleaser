package effectiveconfig

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/amane3/goreleaser/internal/testlib"
	"github.com/amane3/goreleaser/pkg/config"
	"github.com/amane3/goreleaser/pkg/context"
	"github.com/stretchr/testify/require"
)

func TestPipeDescription(t *testing.T) {
	require.NotEmpty(t, Pipe{}.String())
}

func Test(t *testing.T) {
	var folder = testlib.Mktmp(t)
	dist := filepath.Join(folder, "dist")
	require.NoError(t, os.Mkdir(dist, 0755))
	var ctx = context.New(
		config.Project{
			Dist: dist,
		},
	)
	require.NoError(t, Pipe{}.Run(ctx))
	bts, err := ioutil.ReadFile(filepath.Join(dist, "config.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, string(bts))
}
