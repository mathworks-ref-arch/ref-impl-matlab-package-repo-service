# Copyright 2026 The MathWorks, Inc.

FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -o /mpm-repo-server ./cmd/mpm-repo-server

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=build /mpm-repo-server /mpm-repo-server
COPY actions/ /actions/
COPY configs/ /configs/
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/mpm-repo-server", "--config", "/configs/server-artifactory.json"]
