// Package ptr menyediakan helper pointer kecil yang sering dipakai DTO.
package ptr

// To mengembalikan pointer dari nilai apa pun.
func To[T any](v T) *T { return &v }

// Value mengembalikan nilai yang ditunjuk, atau zero value bila nil.
func Value[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// ValueOr mengembalikan nilai yang ditunjuk, atau fallback bila nil.
func ValueOr[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}
