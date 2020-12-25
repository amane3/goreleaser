package client

import (
	"github.com/amane3/goreleaser/pkg/config"
)

func RepoFromRef(ref config.RepoRef) Repo {
	return Repo{
		Owner: ref.Owner,
		Name:  ref.Name,
	}
}
