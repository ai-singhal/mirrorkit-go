package mirrorkit

import "log"

// logger emits internal debug lines only when enabled. The collector never logs
// above debug — it must stay silent and safe inside the host app.
type logger struct{ enabled bool }

func (l *logger) debugf(format string, args ...any) {
	if l != nil && l.enabled {
		log.Printf("[mirrorkit] "+format, args...)
	}
}
