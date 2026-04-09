FROM golang:1.24 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.buildVersion=${VERSION}" \
    -o /out/myapp ./cmd/myapp

FROM gcr.io/distroless/static-debian12
WORKDIR /
COPY --from=builder /out/myapp /myapp

EXPOSE 8080
ENTRYPOINT ["/myapp"]