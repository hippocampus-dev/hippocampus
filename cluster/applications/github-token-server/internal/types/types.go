package types

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

func (b *Body) ResolveProfile(p profile) {
	if b.Permissions == nil {
		b.Permissions = make(map[string]string)
	}

	switch p {
	case Reader:
		b.Permissions["contents"] = "read"
		b.Permissions["metadata"] = "read"
	case Writer:
		b.Permissions["contents"] = "write"
		b.Permissions["metadata"] = "read"
	}
}
