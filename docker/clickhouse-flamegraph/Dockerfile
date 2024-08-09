FROM --platform=$BUILDPLATFORM golang:alpine AS builder
ARG TARGETPLATFORM
LABEL maintainer="Eugene Klimov <bloodjazman@gmail.com>"
USER root
COPY ./ /go/src/github.com/Slach/clickhouse-flamegraph
WORKDIR /go/src/github.com/Slach/clickhouse-flamegraph
RUN go mod tidy
RUN --mount=type=cache,id=clickhouse-flamegraph-gobuild,target=/root/ go mod download -x
RUN --mount=type=cache,id=clickhouse-flamegraph-gobuild,target=/root/ GOOS=$( echo ${TARGETPLATFORM} | cut -d "/" -f 1) GOARCH=$( echo ${TARGETPLATFORM} | cut -d "/" -f 2) go build -o /usr/bin/clickhouse-flamegraph main.go
RUN apk --no-cache add git
RUN git clone https://github.com/brendangregg/FlameGraph.git /opt/flamegraph/

FROM alpine:latest
RUN apk --no-cache add bash tzdata perl
COPY --from=builder /opt/flamegraph /opt/flamegraph
RUN ln -vsf /opt/flamegraph/flamegraph.pl /usr/bin/flamegraph.pl
COPY --from=builder /usr/bin/clickhouse-flamegraph /usr/bin/clickhouse-flamegraph
COPY docker/clickhouse-flamegraph/entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
