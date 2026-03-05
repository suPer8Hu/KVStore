package store

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func BenchmarkSet(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := New(ctx, 1*time.Second, 10*time.Millisecond, 10000000)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := "key_" + strconv.Itoa(i)
		_ = s.Set(ctx, key, "value", 1*time.Minute)
	}
}

func BenchmarkGet(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := New(ctx, 1*time.Second, time.Nanosecond, 10000000)

	testKey := "test_key"
	_ = s.Set(ctx, testKey, "test_value", 1*time.Minute)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, _ = s.Get(ctx, testKey)
	}
}
