package telemetry

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// InitTracer 初始化并返回一个 TracerProvider
func InitTracer(serviceName string) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	// 1. 设置 Exporter 到 OTLP HTTP endpoint
	// 默认本地 collector: localhost:4318，可通过环境变量覆盖
	opts := []otlptracehttp.Option{
		otlptracehttp.WithInsecure(),
	}

	if endpointURL := os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"); endpointURL != "" {
		opts = append(opts, otlptracehttp.WithEndpointURL(endpointURL))
	} else {
		opts = append(opts, otlptracehttp.WithEndpoint("localhost:4318"))
	}

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	// 2. 设置 Resource (给你的服务起个名字，在 Jaeger 里按这个名字搜索)
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
	)

	// 3. 创建 TracerProvider，并将刚才设置的 exporter 和 resource 传进去
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter), // Batcher 会将短时间内的 trace 打包一起发送，性能更好
		sdktrace.WithResource(res),
	)

	// 4. 设置全局 TracerProvider
	// 这样在项目的任何地方调用 otel.Tracer("xxx") 都能获取到这个实例
	otel.SetTracerProvider(tp)

	// 5. 设置全局 Propagator：TraceContext + Baggage
	// 这是分布式追踪的核心！它决定了 Trace ID 如何在 HTTP 请求头中传递
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}
