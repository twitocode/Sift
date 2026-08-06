package common

func Map[T any](slice []T, f func(e T) bool) []T {
	result := make([]T, 0, len(slice))
	for _, e := range slice {
		if f(e) {
			result = append(result, e)
		}
	}

	return result
}
