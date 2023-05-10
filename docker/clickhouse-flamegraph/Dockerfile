FROM golang:alpine AS builder
MAINTAINER Eugene Klimov <bloodjazman@gmail.com>

COPY ./ /go/src/github.com/Slach/clickhouse-flamegraph
WORKDIR /go/src/github.com/Slach/clickhouse-flamegraph
RUN go mod tidy
RUN go build -o /usr/bin/clickhouse-flamegraph main.go
RUN apk --no-cache add git
RUN git clone https://github.com/brendangregg/FlameGraph.git /opt/flamegraph/

FROM alpine:3.18
RUN apk --no-cache add bash tzdata perl
COPY --from=builder /opt/flamegraph /opt/flamegraph
RUN ln -vsf /opt/flamegraph/flamegraph.pl /usr/bin/flamegraph.pl
COPY --from=builder /usr/bin/clickhouse-flamegraph /usr/bin/clickhouse-flamegraph
COPY docker/clickhouse-flamegraph/entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
