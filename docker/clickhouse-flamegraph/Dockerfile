FROM golang:alpine AS builder
MAINTAINER Eugene Klimov <bloodjazman@gmail.com>

COPY ./ /go/src/github.com/Slach/clickhouse-flamegraph
WORKDIR /go/src/github.com/Slach/clickhouse-flamegraph
RUN go mod tidy
RUN go build -o /usr/bin/clickhouse-flamegraph main.go

FROM alpine:3
RUN apk --no-cache add bash tzdata git perl
RUN git clone https://github.com/brendangregg/FlameGraph.git /opt/flamegraph/
RUN ln -vsf /opt/flamegraph/flamegraph.pl /usr/bin/flamegraph.pl
COPY --from=builder /usr/bin/clickhouse-flamegraph /usr/bin/clickhouse-flamegraph
COPY docker/clickhouse-flamegraph/entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]