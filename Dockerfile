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

EXPOSE 8080

CMD /coco-aggregate-healthcheck \
	--etcd-peers "$ETCD_PEERS" \
	--key-prefix "$KEY_PREFIX" \
	--vulcand "$VULCAND_ADDRESS" \
	--socks-proxy "$SOCKS_PROXY"

