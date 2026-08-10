package applog

import "context"

type ctxKey int

const uidKey ctxKey = 1

// ContextWithUID returns a child context carrying the authenticated uid for file logs.
func ContextWithUID(ctx context.Context, uid string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, uidKey, uid)
}

// UIDFromContext returns the uid injected by ContextWithUID.
func UIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	v, ok := ctx.Value(uidKey).(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}
