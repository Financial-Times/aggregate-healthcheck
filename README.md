# Coco Aggregate Healthcheck Service

Looks up the services exposed through vulcand, calls their /__health endpoint and reports back in the standard FT Healthcheck format.

## Building

```
CGO_ENABLED=0 go build -a -installsuffix cgo -o coco-aggregate-healthcheck .

docker build -t coco/coco-aggregate-healthcheck .
```

## Running

```
docker run \
    --env ETCD_PEERS=http://localhost:2379 \
    --env VULCAND_ADDRESS=localhost:8080 \
    --env KEY_PREFIX=/ft/healthcheck \
    coco/coco-aggregate-healthcheck
```

Binary
```
ssh -D 2323 -N core@$FLEETCTL_TUNNEL
./coco-aggregate-healthcheck --socks-proxy localhost:2323
```
