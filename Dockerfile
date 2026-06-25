FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 go build -o linksmith ./cmd/linksmith

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/linksmith .
EXPOSE 8080
VOLUME ["/app/data"]
ENV LINKSMITH_DB=/app/data/linksmith.json
CMD ["./linksmith"]
