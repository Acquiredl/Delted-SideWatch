FROM golang:1.25-alpine AS builder

WORKDIR /build
COPY services/manager/go.mod services/manager/go.sum ./
RUN go mod download
COPY services/manager/ ./
RUN CGO_ENABLED=0 go build -o manager ./cmd/manager/

FROM alpine:3.19
RUN adduser -D -u 1000 appuser
COPY --from=builder /build/manager /usr/local/bin/manager
USER appuser
EXPOSE 8081 9090
ENTRYPOINT ["manager"]
