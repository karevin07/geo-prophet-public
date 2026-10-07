FROM golang:1.26-alpine AS builder
WORKDIR /src
RUN apk add --no-cache build-base zlib-dev zlib-static
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w -linkmode external -extldflags -static" -o /out/api ./cmd/server
# The importer also uses h3 (cgo) to derive district boundaries.
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w -linkmode external -extldflags -static" -o /out/importer ./cmd/importer

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/api /api
COPY --from=builder /out/importer /importer
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
