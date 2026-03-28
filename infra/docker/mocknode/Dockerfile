FROM golang:1.25-alpine AS build

WORKDIR /app
COPY services/mocknode/go.mod ./
COPY services/mocknode/ ./
RUN go build -o /mocknode .

FROM alpine:3.19
COPY --from=build /mocknode /mocknode
EXPOSE 3333 18081
CMD ["/mocknode"]
