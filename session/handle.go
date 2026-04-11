package session

// SessionHandle identifies one logical chat (same IM source + session key).
type SessionHandle struct {
	Source     string
	SessionKey string
}

func (h SessionHandle) key() string {
	src := h.Source
	if h.SessionKey == "" {
		return src + "\x00__default__"
	}
	return src + "\x00" + h.SessionKey
}
