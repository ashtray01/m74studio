package decoder

import (
	"fmt"
	"sort"
)

func medianInterval(frames []Frame) float64 {
	d := make([]float64, 0, len(frames)-1)
	for i := 1; i < len(frames); i++ {
		d = append(d, frames[i].T-frames[i-1].T)
	}
	sort.Float64s(d)
	n := len(d)
	if n%2 == 1 {
		return d[n/2]
	}
	return (d[n/2-1] + d[n/2]) / 2
}
func (s *Session) addTransitions() {
	ids := map[string]bool{"p043": true, "p049": true, "p050": true, "p059": true, "p065": true}
	for i := 1; i < len(s.Frames); i++ {
		a, b := s.Frames[i-1], s.Frames[i]
		if b.T-a.T > 2 {
			continue
		}
		for j, p := range s.Parameters {
			if a.Values[j] == nil || b.Values[j] == nil {
				continue
			}
			old, v := *a.Values[j], *b.Values[j]
			message := ""
			if p.ID == "p004" {
				if old == 0 && v > 0 {
					message = "Появились обороты двигателя"
				}
				if old > 0 && v == 0 {
					message = "Обороты двигателя снизились до нуля"
				}
			}
			if ids[p.ID] && v != old {
				state := "выкл."
				if v != 0 {
					state = "вкл."
				}
				message = fmt.Sprintf("%s: %s", p.Name, state)
			}
			if message != "" {
				s.Events = append(s.Events, Event{i, b.Time, b.T, "state", message})
			}
		}
	}
	sort.SliceStable(s.Events, func(i, j int) bool { return s.Events[i].T < s.Events[j].T })
}
