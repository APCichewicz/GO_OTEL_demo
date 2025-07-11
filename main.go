package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
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

func init() {
	if err := godotenv.Load(".env.example"); err != nil {
		log.Printf("No .env file found")
	}
}

func newResource() (*resource.Resource, error) {
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(os.Getenv("SERVICE_NAME")),
			semconv.ServiceVersion(os.Getenv("SERVICE_VERSION")),
			semconv.DeploymentEnvironment(os.Getenv("ENVIRONMENT")),
			semconv.HostName(os.Getenv("HOSTNAME")),
		),
	)
	if err != nil {
		log.Printf("Failed to create resource: %v", err)
		return nil, err
	}
	return r, nil
}

func newExporter() (*stdouttrace.Exporter, error) {
	exporter, err := stdouttrace.New(
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		log.Printf("Failed to create exporter: %v", err)
		return nil, err
	}
	return exporter, nil
}
func newTracerProvider(exp sdktrace.SpanExporter, res *resource.Resource) *sdktrace.TracerProvider {
	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
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
	res, err := newResource()
	if err != nil {
		log.Fatal("Failed to create resource")
	}
	exp, err := newExporter()
	if err != nil {
		log.Fatal("Failed to create exporter")
	}
	tp := newTracerProvider(exp, res)
	defer func() { _ = tp.Shutdown(ctx) }()
	otel.SetTracerProvider(tp)

	tracer = tp.Tracer("scratch")

	api := api.NewServer(tracer, queries)
	api.Start()

}
