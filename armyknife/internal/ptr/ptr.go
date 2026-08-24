package ptr

func To[T any](value T) *T {
	return &value
}
