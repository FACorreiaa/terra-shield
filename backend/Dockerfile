# syntax=docker/dockerfile:1.7
# One image, two releases: the api runs `shield serve` (default CMD), the
# worker release overrides the command with `shield worker`.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY --link go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download
COPY --link cmd ./cmd
COPY --link internal ./internal
COPY --link db ./db
COPY --link api ./api
ARG GIT_SHA=unknown
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.gitSHA=${GIT_SHA}" -o /out/shield ./cmd/shield

# distroless/static carries CA roots (outbound TLS to the feeds) and tzdata.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --link --from=build /out/shield /app/shield
WORKDIR /app
EXPOSE 8080 9090
USER nonroot:nonroot
ENTRYPOINT ["/app/shield"]
CMD ["serve"]
