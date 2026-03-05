package main

import (
	"KVStore/store"
	"KVStore/telemetry"
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type scenarioResult struct {
	label      string
	strategy   store.LockStrategy
	shardCount int

	createSuccess int
	createFail    int
	getSuccess    int
	getMiss       int
	getFail       int
	deleteSuccess int
	deleteFail    int

	totalWait  time.Duration
	avgWait    time.Duration
	waitSample int64
	duration   time.Duration
}

func main() {
	tp, err := telemetry.InitTracer("my-kv-store")
	if err != nil {
		log.Fatalf("初始化 Tracer 失败: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("关闭 Tracer 失败: %v", err)
		}
	}()

	tracer := otel.Tracer("kvstore/main")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx, rootSpan := tracer.Start(ctx, "kvstore.lock_strategy_comparison")
	defer rootSpan.End()

	totalRequests := 100
	fmt.Println("🌊 开始锁竞争对比压测：before=global lock, after=sharded locks")

	before := runScenario(ctx, tracer, "before", totalRequests, store.LockStrategyGlobal, 1)
	after := runScenario(ctx, tracer, "after", totalRequests, store.LockStrategySharded, 16)

	improvement := improvementPercent(before.totalWait, after.totalWait)
	rootSpan.SetAttributes(
		attribute.Int("experiment.requests", totalRequests),
		attribute.Float64("compare.before_wait_ms", durationToMS(before.totalWait)),
		attribute.Float64("compare.after_wait_ms", durationToMS(after.totalWait)),
		attribute.Float64("compare.wait_improvement_pct", improvement),
	)

	fmt.Printf("\n📊 锁策略压测报告\n")
	printScenario(before)
	printScenario(after)
	fmt.Printf("[Compare] mutex_wait_total_improvement=%.2f%%\n", improvement)
	fmt.Println("Jaeger 里看这两条 trace：kvstore.scenario.before / kvstore.scenario.after")
}

func runScenario(
	ctx context.Context,
	tracer trace.Tracer,
	label string,
	totalRequests int,
	strategy store.LockStrategy,
	shardCount int,
) scenarioResult {
	scenarioName := fmt.Sprintf("kvstore.scenario.%s", label)
	scenarioCtx, scenarioSpan := tracer.Start(ctx, scenarioName)
	scenarioStart := time.Now()

	// 为了更清晰地观察锁竞争，避免限流成为主瓶颈
	s := store.NewWithLockStrategy(
		scenarioCtx,
		1*time.Second,
		time.Microsecond,
		totalRequests*10,
		strategy,
		shardCount,
	)

	scenarioSpan.SetAttributes(
		attribute.String("scenario.label", label),
		attribute.String("kv.lock.strategy", string(strategy)),
		attribute.Int("kv.shard.count", s.ShardCount()),
		attribute.Int("scenario.requests", totalRequests),
	)

	createCtx, createPhaseSpan := tracer.Start(scenarioCtx, "kvstore.phase.create")
	createSuccess, createFail := runSetPhase(createCtx, tracer, s, totalRequests, 15*time.Second)
	createPhaseSpan.SetAttributes(
		attribute.Int("phase.requests", totalRequests),
		attribute.Int("phase.success", createSuccess),
		attribute.Int("phase.failed", createFail),
	)
	createPhaseSpan.End()

	getCtx, getPhaseSpan := tracer.Start(scenarioCtx, "kvstore.phase.get")
	getSuccess, getMiss, getFail := runGetPhase(getCtx, tracer, s, totalRequests)
	getPhaseSpan.SetAttributes(
		attribute.Int("phase.requests", totalRequests),
		attribute.Int("phase.success", getSuccess),
		attribute.Int("phase.miss", getMiss),
		attribute.Int("phase.failed", getFail),
	)
	getPhaseSpan.End()

	deleteCtx, deletePhaseSpan := tracer.Start(scenarioCtx, "kvstore.phase.delete")
	deleteSuccess, deleteFail := runDeletePhase(deleteCtx, tracer, s, totalRequests)
	deletePhaseSpan.SetAttributes(
		attribute.Int("phase.requests", totalRequests),
		attribute.Int("phase.success", deleteSuccess),
		attribute.Int("phase.failed", deleteFail),
	)
	deletePhaseSpan.End()

	totalWait, avgWait, waitSamples := s.WriteLockWaitStats()
	scenarioDuration := time.Since(scenarioStart)
	scenarioSpan.SetAttributes(
		attribute.Int("create.success", createSuccess),
		attribute.Int("create.fail", createFail),
		attribute.Int("get.success", getSuccess),
		attribute.Int("get.miss", getMiss),
		attribute.Int("get.fail", getFail),
		attribute.Int("delete.success", deleteSuccess),
		attribute.Int("delete.fail", deleteFail),
		attribute.Int64("kv.mutex.wait_samples", waitSamples),
		attribute.Float64("kv.mutex.wait_total_ms", durationToMS(totalWait)),
		attribute.Float64("kv.mutex.wait_avg_ms", durationToMS(avgWait)),
		attribute.Float64("scenario.duration_ms", durationToMS(scenarioDuration)),
	)
	scenarioSpan.End()

	return scenarioResult{
		label: label,

		strategy:   strategy,
		shardCount: s.ShardCount(),

		createSuccess: createSuccess,
		createFail:    createFail,
		getSuccess:    getSuccess,
		getMiss:       getMiss,
		getFail:       getFail,
		deleteSuccess: deleteSuccess,
		deleteFail:    deleteFail,

		totalWait:  totalWait,
		avgWait:    avgWait,
		waitSample: waitSamples,
		duration:   scenarioDuration,
	}
}

func printScenario(res scenarioResult) {
	fmt.Printf("[%s] strategy=%s, shards=%d, duration=%.3fms\n",
		res.label, res.strategy, res.shardCount, durationToMS(res.duration))
	fmt.Printf("  Create: success=%d fail=%d\n", res.createSuccess, res.createFail)
	fmt.Printf("  Get:    success=%d miss=%d fail=%d\n", res.getSuccess, res.getMiss, res.getFail)
	fmt.Printf("  Delete: success=%d fail=%d\n", res.deleteSuccess, res.deleteFail)
	fmt.Printf("  Mutex:  wait_total=%.3fms avg=%.3fms samples=%d\n",
		durationToMS(res.totalWait), durationToMS(res.avgWait), res.waitSample)
}

func improvementPercent(before time.Duration, after time.Duration) float64 {
	if before <= 0 {
		return 0
	}
	return (float64(before-after) / float64(before)) * 100
}

func durationToMS(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

func runSetPhase(ctx context.Context, tracer trace.Tracer, s *store.KVStore, total int, ttl time.Duration) (success int, fail int) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 1; i <= total; i++ {
		wg.Add(1)
		go func(id int, parentCtx context.Context) {
			defer wg.Done()
			reqCtx, span := tracer.Start(parentCtx, "kvstore.request.set")
			defer span.End()

			key := fmt.Sprintf("key_%d", id)
			val := fmt.Sprintf("val_%d", id)
			span.SetAttributes(
				attribute.Int("request.id", id),
				attribute.String("kv.key", key),
			)

			err := s.Set(reqCtx, key, val, ttl)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fail++
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				span.SetAttributes(attribute.Bool("request.success", false))
				return
			}
			success++
			span.SetAttributes(attribute.Bool("request.success", true))
		}(i, ctx)
	}

	wg.Wait()
	return success, fail
}

func runGetPhase(ctx context.Context, tracer trace.Tracer, s *store.KVStore, total int) (success int, miss int, fail int) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 1; i <= total; i++ {
		wg.Add(1)
		go func(id int, parentCtx context.Context) {
			defer wg.Done()
			reqCtx, span := tracer.Start(parentCtx, "kvstore.request.get")
			defer span.End()

			key := fmt.Sprintf("key_%d", id)
			span.SetAttributes(
				attribute.Int("request.id", id),
				attribute.String("kv.key", key),
			)

			_, found, err := s.Get(reqCtx, key)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fail++
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				span.SetAttributes(attribute.Bool("request.success", false))
				return
			}
			if !found {
				miss++
				span.SetAttributes(
					attribute.Bool("request.success", false),
					attribute.Bool("request.miss", true),
					attribute.Bool("kv.found", false),
				)
				return
			}
			success++
			span.SetAttributes(
				attribute.Bool("request.success", true),
				attribute.Bool("request.miss", false),
				attribute.Bool("kv.found", true),
			)
		}(i, ctx)
	}

	wg.Wait()
	return success, miss, fail
}

func runDeletePhase(ctx context.Context, tracer trace.Tracer, s *store.KVStore, total int) (success int, fail int) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 1; i <= total; i++ {
		wg.Add(1)
		go func(id int, parentCtx context.Context) {
			defer wg.Done()
			reqCtx, span := tracer.Start(parentCtx, "kvstore.request.delete")
			defer span.End()

			key := fmt.Sprintf("key_%d", id)
			span.SetAttributes(
				attribute.Int("request.id", id),
				attribute.String("kv.key", key),
			)

			err := s.Delete(reqCtx, key)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fail++
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				span.SetAttributes(attribute.Bool("request.success", false))
				return
			}
			success++
			span.SetAttributes(attribute.Bool("request.success", true))
		}(i, ctx)
	}

	wg.Wait()
	return success, fail
}
