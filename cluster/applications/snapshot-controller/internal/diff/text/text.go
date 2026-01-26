package text

type DiffResult struct {
	Diff       []byte
	DiffAmount float64
}

type Differ interface {
	Calculate(baseline []byte, target []byte) (*DiffResult, error)
}
