package selfcheck

import "time"

type FlowClock struct{ Now time.Time }

func (c FlowClock) Current() time.Time {
	if c.Now.IsZero() {
		return time.Now().UTC()
	}
	return c.Now
}
