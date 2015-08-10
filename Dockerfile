FROM gliderlabs/alpine:3.2

ENV ETCD_PEERS http://localhost:4001
ENV KEY_PREFIX /services
ENV VULCAND_ADDRESS localhost:8080

ADD coco-aggregate-healthcheck /coco-aggregate-healthcheck

EXPOSE 8080

CMD /coco-aggregate-healthcheck \
	--etcd-peers "$ETCD_PEERS" \
	--key-prefix "$KEY_PREFIX" \
	--vulcand "$VULCAND_ADDRESS" \
	--socks-proxy "$SOCKS_PROXY"

