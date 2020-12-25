// Package sourcearchive archives the source of the project using git-archive.
package sourcearchive

import (
	"path/filepath"

	"github.com/amane3/goreleaser/internal/artifact"
	"github.com/amane3/goreleaser/internal/git"
	"github.com/amane3/goreleaser/internal/pipe"
	"github.com/amane3/goreleaser/internal/tmpl"
	"github.com/amane3/goreleaser/pkg/context"
	"github.com/apex/log"
)

// Pipe for source archive.
type Pipe struct{}

func (Pipe) String() string {
	return "creating source archive"
}

// Run the pipe.
func (Pipe) Run(ctx *context.Context) (err error) {
	if !ctx.Config.Source.Enabled {
		return pipe.Skip("source pipe is disabled")
	}

	name, err := tmpl.New(ctx).Apply(ctx.Config.Source.NameTemplate)
	if err != nil {
		return err
	}
	var filename = name + "." + ctx.Config.Source.Format
	var path = filepath.Join(ctx.Config.Dist, filename)
	log.WithField("file", filename).Info("creating source archive")
	out, err := git.Clean(git.Run("archive", "-o", path, ctx.Git.FullCommit))
	log.Debug(out)
	ctx.Artifacts.Add(&artifact.Artifact{
		Type: artifact.UploadableSourceArchive,
		Name: filename,
		Path: path,
		Extra: map[string]interface{}{
			"Format": ctx.Config.Source.Format,
		},
	})
	return err
}

// Default sets the pipe defaults.
func (Pipe) Default(ctx *context.Context) error {
	var archive = &ctx.Config.Source
	if archive.Format == "" {
		archive.Format = "tar.gz"
	}

	if archive.NameTemplate == "" {
		archive.NameTemplate = "{{ .ProjectName }}-{{ .Version }}"
	}
	return nil
}
