package handler

type Handler interface {
	Call(*AlertManagerRequest) error
}
