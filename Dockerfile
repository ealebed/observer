FROM golang:1.24 as build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X github.com/ealebed/observer/internal/version.Version=${VERSION}" -o /out/observer ./cmd/observer

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/observer /observer
USER 65532:65532
ENTRYPOINT ["/observer"]
