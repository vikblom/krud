FROM postgres:bullseye as krud-psql-img

COPY ./initdb/ /docker-entrypoint-initdb.d/


FROM golang:1.17-alpine as krud-http-img

WORKDIR /app
# Allow a layer to be cached just with deps.
COPY go.mod go.sum /app/
RUN go mod download

COPY . /app
# FIXME: Caching not kicking in with current config of docker.
# # syntax = docker/dockerfile:1-experimental
# RUN --mount=type=cache,target=/root/.cache/go-build go build -o main ./cmd/main.go
RUN go build -o main ./cmd/main.go

ENTRYPOINT ["/app/main"]
