package auth

import "context"

type principalKey struct{}

// WithPrincipal menempelkan Principal ke context request.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFromContext mengambil Principal dari context (ok=false bila tidak ada).
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// MustPrincipal mengambil Principal dan panik bila tidak ada. Hanya dipakai
// pada handler yang sudah dipastikan berada di belakang middleware auth.
func MustPrincipal(ctx context.Context) Principal {
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		panic("auth: principal tidak ada pada context")
	}
	return p
}
