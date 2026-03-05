package limiter

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type RateLimiter struct {
	tokens chan struct{}
}

var tracer = otel.Tracer("kvstore/limiter")

func NewLimiter(ctx context.Context, rate time.Duration, capacity int) *RateLimiter {
	channel := make(chan struct{}, capacity)
	// 塞满桶，形容目前有处理请求的能力
	for i := 0; i < capacity; i++ {
		channel <- struct{}{}
	}

	rl := &RateLimiter{
		tokens: channel,
	}

	go func() {
		ticker := time.NewTicker(rate)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case channel <- struct{}{}: // 桶没满，成功塞入一个令牌！
				default: // 桶满了，塞不进去了
				}
			}
		}
	}()

	return rl
}

func (rl *RateLimiter) Allow(ctx context.Context) bool {
	_, span := tracer.Start(ctx, "ratelimiter.allow")
	defer span.End()

	select {
	case <-rl.tokens:
		span.SetAttributes(attribute.Bool("ratelimiter.allowed", true))
		return true
	default:
		span.SetAttributes(attribute.Bool("ratelimiter.allowed", false))
		return false
	}
}
