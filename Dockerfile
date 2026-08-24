# Build stage
FROM golang:1.25-bookworm AS build

WORKDIR /app

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

# No go.sum is committed (nothing to generate it with outside a real Go
# toolchain) — `go mod tidy` resolves every import in ./... and writes an
# accurate go.mod/go.sum right here, in a build environment that actually
# has network access, then the build below uses exactly those pinned versions.
RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server ./cmd/server

# Final stage — static binary, no JDK/JRE, minimal base image.
FROM gcr.io/distroless/static-debian12

WORKDIR /app
COPY --from=build /app/server /app/server

EXPOSE 8080

ENTRYPOINT ["/app/server"]
