FROM gliderlabs/alpine:3.2

ADD . /aggregate-healthcheck
RUN apk --update add go git\
  && export GOPATH=/.gopath \
  && go get github.com/Financial-Times/coco-aggregate-healthcheck \
  && cd aggregate-healthcheck \
  && go build \
  && mv aggregate-healthcheck /coco-aggregate-healthcheck \
  && apk del go git \
  && rm -rf $GOPATH /var/cache/apk/*

ENV ETCD_PEERS http://localhost:4001
ENV KEY_PREFIX /services
ENV VULCAND_ADDRESS localhost:8080
ENV GRAPHITE_HOST graphite.ft.com
ENV GRAPHITE_PORT 2003
ENV ENVIRONMENT local

EXPOSE 8080

CMD /coco-aggregate-healthcheck \
	--etcd-peers "$ETCD_PEERS" \
	--vulcand "$VULCAND_ADDRESS" \
	--socks-proxy "$SOCKS_PROXY" \
    --graphite-host "$GRAPHITE_HOST" \
    --graphite-port "$GRAPHITE_PORT" \
    --environment "$ENVIRONMENT"
