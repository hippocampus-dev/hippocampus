package jsonnet

import (
	"github.com/google/go-jsonnet"
	"golang.org/x/xerrors"
)

// Renderer renders Jsonnet to JSON
type Renderer struct {
	vm *jsonnet.VM
}

// NewRenderer creates a new Jsonnet renderer
func NewRenderer() *Renderer {
	vm := jsonnet.MakeVM()
	return &Renderer{vm: vm}
}

// Render evaluates Jsonnet code and returns JSON
func (r *Renderer) Render(source string) ([]byte, error) {
	if source == "" {
		return nil, xerrors.New("empty jsonnet source")
	}

	result, err := r.vm.EvaluateAnonymousSnippet("dashboard.jsonnet", source)
	if err != nil {
		return nil, xerrors.Errorf("failed to evaluate jsonnet: %w", err)
	}

	return []byte(result), nil
}
