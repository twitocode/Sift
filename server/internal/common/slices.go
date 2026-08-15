package common

import "slices"

func Map[T any, O any](slice []T, f func(e T, i int) (O, bool)) []O {
	result := make([]O, 0, len(slice))
	for i, e := range slice {
		if o, ok := f(e, i); ok {
			result = append(result, o)
		}
	}

	return result
}

func ToStructMap[T comparable, K comparable](s []T, f func(e T) K) map[K]struct{} {
	out := make(map[K]struct{})

	for _, e := range s {
		key := f(e)
		out[key] = struct{}{}
	}

	return out
}

func ToMap[T comparable, K comparable, V any](s []T, f func(e T) (K, V)) map[K]V {
	out := make(map[K]V)

	for _, e := range s {
		key, value := f(e)
		out[key] = value
	}

	return out
}

func Intersection[T comparable](s1 []T, s2 []T) []T {
	out := make([]T, 0)

	var s []T
	var p []T

	//gets worse as s2 size gets closer to s1, vice versa but whatever
	if len(s1) < len(s2) {
		s = s1
		p = s2
	} else {
		s = s2
		p = s1
	}

	for _, e := range s {
		if slices.Contains(p, e) {
			out = append(out, e)
		}
	}

	return out
}

func Union[T comparable](s1 []T, s2 []T) []T {
	out := make([]T, 0)

	var s []T
	var p []T

	//gets worse as s2 size gets closer to s1, vice versa but whatever
	if len(s1) < len(s2) {
		s = s1
		p = s2
	} else {
		s = s2
		p = s1
	}

	for _, e := range s {
		out = append(out, e)
	}

	for _, e := range p {
		if !slices.Contains(out, e) {
			out = append(out, e)
		}
	}

	return out
}
