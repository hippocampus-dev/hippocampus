package text

type DiffResult struct {
	Diff      []byte
	DiffRatio float64
}

type Differ interface {
	Calculate(baseline []byte, target []byte) (*DiffResult, error)
}
