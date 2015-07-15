# Coco Aggregate Healthcheck Service

Looks up the services exposed through vulcand, calls their /__health endpoint and reports back in the standard FT Healthcheck format.

## Building

```
docker build -t coco/coco-aggregate-healthcheck
```

## Running

```
docker run \
    --env ETCD_PEERS=http://localhost:4001 \
    --env VULCAND_ADDRESS=localhost:8080 \
    --env KEY_PREFIX=/vulcand/frontends \
    --env HOSTNAME=cluster-elb-1694467668.eu-west-1.elb.amazonaws.com \
    coco/coco-aggregate-healthcheck
```

Binary
```
ssh -D 2323 -N core@$FLEETCTL_TUNNEL
./coco-aggregate-healthcheck --socks-proxy localhost:2323
```
