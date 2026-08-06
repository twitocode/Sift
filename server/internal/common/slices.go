package common

func Map[T any, O any](slice []T, f func(e T) (O, bool)) []O {
	result := make([]O, 0, len(slice))
	for _, e := range slice {
		if o, ok := f(e); ok {
			result = append(result, o)
		}
	}

	return result
}
