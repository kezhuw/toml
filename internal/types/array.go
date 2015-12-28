package types

import "time"

func (v *Array) Interface() []interface{} {
	a := make([]interface{}, len(v.Elems))
	for i, value := range v.Elems {
		switch value := value.(type) {
		case Boolean:
			a[i] = bool(value)
		case Integer:
			a[i] = int64(value)
		case Float:
			a[i] = float64(value)
		case String:
			a[i] = string(value)
		case Datetime:
			a[i] = time.Time(value)
		case *Array:
			a[i] = value.Interface()
		case *Table:
			a[i] = value.Interface()
		}
	}
	return a
}
