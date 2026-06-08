package notify

type Event struct {
	Type    string
	Source  string
	Payload any
}

type eventHandler struct {
	pattern string
	handler Handler
}

type Handler func(Event)

type Hub struct {
	handlers []eventHandler
}

var global = newHub()

func newHub() *Hub {
	return &Hub{}
}

func Global() *Hub {
	return global
}

func (h *Hub) On(pattern string, handler Handler) {
	h.handlers = append(h.handlers, eventHandler{pattern: pattern, handler: handler})
}

func (h *Hub) Emit(ev Event) {
	for _, eh := range h.handlers {
		if match(eh.pattern, ev.Type) {
			eh.handler(ev)
		}
	}
}

func match(pattern, typ string) bool {
	if pattern == "*" {
		return true
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' && pattern[len(pattern)-2] == ':' {
		prefix := pattern[:len(pattern)-1]
		if len(typ) >= len(prefix) && typ[:len(prefix)] == prefix {
			return true
		}
	}
	return pattern == typ
}

func On(pattern string, handler Handler) {
	global.On(pattern, handler)
}

func Emit(ev Event) {
	global.Emit(ev)
}
