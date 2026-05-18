FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /uapi ./cmd/uapi/

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /uapi /app/uapi
WORKDIR /app
EXPOSE 8080
ENTRYPOINT ["/app/uapi"]
CMD ["-config", "config.yaml"]
