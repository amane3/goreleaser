// Package defaults make the list of Defaulter implementations available
// so projects extending GoReleaser are able to use it, namely, GoDownloader.
package defaults

import (
	"fmt"

	"github.com/amane3/goreleaser/internal/pipe/archive"
	"github.com/amane3/goreleaser/internal/pipe/artifactory"
	"github.com/amane3/goreleaser/internal/pipe/blob"
	"github.com/amane3/goreleaser/internal/pipe/brew"
	"github.com/amane3/goreleaser/internal/pipe/build"
	"github.com/amane3/goreleaser/internal/pipe/checksums"
	"github.com/amane3/goreleaser/internal/pipe/docker"
	"github.com/amane3/goreleaser/internal/pipe/milestone"
	"github.com/amane3/goreleaser/internal/pipe/nfpm"
	"github.com/amane3/goreleaser/internal/pipe/project"
	"github.com/amane3/goreleaser/internal/pipe/release"
	"github.com/amane3/goreleaser/internal/pipe/scoop"
	"github.com/amane3/goreleaser/internal/pipe/sign"
	"github.com/amane3/goreleaser/internal/pipe/snapcraft"
	"github.com/amane3/goreleaser/internal/pipe/snapshot"
	"github.com/amane3/goreleaser/internal/pipe/sourcearchive"
	"github.com/amane3/goreleaser/pkg/context"
)

// Defaulter can be implemented by a Piper to set default values for its
// configuration.
type Defaulter interface {
	fmt.Stringer

	// Default sets the configuration defaults
	Default(ctx *context.Context) error
}

// Defaulters is the list of defaulters.
// nolint: gochecknoglobals
var Defaulters = []Defaulter{
	snapshot.Pipe{},
	release.Pipe{},
	project.Pipe{},
	build.Pipe{},
	sourcearchive.Pipe{},
	archive.Pipe{},
	nfpm.Pipe{},
	snapcraft.Pipe{},
	checksums.Pipe{},
	sign.Pipe{},
	docker.Pipe{},
	artifactory.Pipe{},
	blob.Pipe{},
	brew.Pipe{},
	scoop.Pipe{},
	milestone.Pipe{},
}
