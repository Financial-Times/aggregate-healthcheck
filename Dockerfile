FROM golang

RUN go get github.com/Financial-Times/coco-aggregate-healthcheck

ENV ETCD_PEERS http://localhost:4001
ENV KEY_PREFIX /vulcand/frontends
ENV VULCAND_ADDRESS localhost:8080

EXPOSE 8080

CMD $GOPATH/bin/coco-aggregate-healthcheck \
	--etcd-peers "$ETCD_PEERS" \
	--key-prefix "$KEY_PREFIX" \
	--vulcand "$VULCAND_ADDRESS" \
	--exclude "$EXCLUDE_SERVICES" \
	--socks-proxy "$SOCKS_PROXY" \
        --elb-hostname "$ELB_HOSTNAME"

