FROM --platform=$BUILDPLATFORM golang:1.25 AS builder
ARG TARGETOS TARGETARCH VERSION BUILD_SHA

WORKDIR /src

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/onyxia-services/main.go ./cmd/onyxia-services/main.go
COPY services/ ./services/
COPY internal/ ./internal/

# Build
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 \
    go build -trimpath -ldflags "-s -w -X=main.Version=${VERSION} -X=main.Build=${BUILD_SHA}" \
    -o /out/app ./cmd/onyxia-services/main.go

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder --chmod=0755 /out/app /
USER 65532:65532

ENTRYPOINT ["/app"]
