package types

import "time"

func (t *Table) Interface() map[string]interface{} {
	m := make(map[string]interface{}, len(t.Elems))
	for key, value := range t.Elems {
		switch value := value.(type) {
		case Boolean:
			m[key] = bool(value)
		case Integer:
			m[key] = int64(value)
		case Float:
			m[key] = float64(value)
		case String:
			m[key] = string(value)
		case Datetime:
			m[key] = time.Time(value)
		case *Array:
			m[key] = value.Interface()
		case *Table:
			m[key] = value.Interface()
		}
	}
	return m
}
