package httpx

import "context"

type requestIDCtxKey string

const (
	requestIDKey requestIDCtxKey = "requestID"
)

func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestIDFromCtx(ctx context.Context) string {
	v := ctx.Value(requestIDKey)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
