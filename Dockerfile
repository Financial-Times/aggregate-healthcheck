FROM golang

RUN go get github.com/Financial-Times/coco-aggregate-healthcheck

ENV ETCD_PEERS http://localhost:4001
ENV KEY_PREFIX /vulcand/frontends

CMD $GOPATH/bin/coco-aggregate-healthcheck \
	--etcd-peers "$ETCD_PEERS" \
	--key-prefix "$KEY_PREFIX" \
	--socks-proxy "$SOCKS_PROXY"