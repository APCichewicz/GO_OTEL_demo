package main

import (
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/apcichewicz/scratch/api"
	"github.com/apcichewicz/scratch/database"
	"github.com/jackc/pgx/v5"
)

var tracer trace.Tracer

func newExporter(ctx context.Context) *stdouttrace.Exporter {
	exporter, err := stdouttrace.New(
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		log.Fatal(err)
	}
	return exporter
}
func newTracerProvider(exp sdktrace.SpanExporter) *sdktrace.TracerProvider {
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("scratch"),
		),
	)
	if err != nil {
		panic(err)
	}
	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(r),
	)
}

func main() {
	db_user := "postgres"
	db_password := "postgres"
	db_host := "localhost"
	db_port := "5432"
	db_name := "postgres"

	conn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s", db_user, db_password, db_host, db_port, db_name)
	db, err := pgx.Connect(context.Background(), conn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close(context.Background())

	queries := database.New(db)

	ctx := context.Background()
	exp := newExporter(ctx)
	tp := newTracerProvider(exp)
	defer func() { _ = tp.Shutdown(ctx) }()
	otel.SetTracerProvider(tp)

	tracer = tp.Tracer("scratch")

	api := api.NewServer(tracer, queries)
	api.Start()

}
