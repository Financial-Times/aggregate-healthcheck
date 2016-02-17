FROM alpine

ADD . /aggregate-healthcheck

ENV ETCD_PEERS=http://localhost:4001 KEY_PREFIX=/services VULCAND_ADDRESS=localhost:8080 GRAPHITE_HOST=graphite.ft.com GRAPHITE_PORT=2003 ENVIRONMENT=local

RUN apk --update add go git\
  && export GOPATH=/.gopath \
  && go get github.com/Financial-Times/aggregate-healthcheck \
  && cd aggregate-healthcheck \
  && go build \
  && mv aggregate-healthcheck /coco-aggregate-healthcheck \
  && apk del go git \
  && rm -rf $GOPATH /var/cache/apk/*

EXPOSE 8080

CMD exec /coco-aggregate-healthcheck \
	--etcd-peers "$ETCD_PEERS" \
	--vulcand "$VULCAND_ADDRESS" \
	--socks-proxy "$SOCKS_PROXY" \
	--graphite-host "$GRAPHITE_HOST" \
	--graphite-port "$GRAPHITE_PORT" \
	--environment "$ENVIRONMENT"
