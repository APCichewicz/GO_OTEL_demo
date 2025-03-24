FROM golang:1.24-alpine

WORKDIR /app

RUN go install github.com/pressly/goose/v3/cmd/goose@latest

COPY ./migrations /app/migrations

ENV GOOSE_DRIVER=postgres
ENV GOOSE_DBSTRING="postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
ENV GOOSE_MIGRATION_DIR=/app/migrations

CMD ["goose", "up"]