package types

import "fmt"

type Body struct {
	Repositories  []string          `json:"repositories,omitempty"`
	RepositoryIds []int             `json:"repository_ids,omitempty"`
	Permissions   map[string]string `json:"permissions,omitempty"`
}

type profile = string

const (
	Reader profile = "reader"
	Writer         = "writer"
)

func (b *Body) ResolveProfile(p profile) error {
	switch p {
	case Reader:
		b.Permissions = map[string]string{
			"contents": "read",
			"metadata": "read",
		}
	case Writer:
		b.Permissions = map[string]string{
			"contents": "write",
			"issues":   "write",
			"metadata": "read",
		}
	default:
		return fmt.Errorf("unknown profile: %s", p)
	}
	return nil
}
