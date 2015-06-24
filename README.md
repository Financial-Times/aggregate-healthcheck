# Coco Aggregate Healthcheck Service

Looks up the services exposed through vulcand, calls their /__health endpoint and reports back in the standard FT Healthcheck format.

## Building

```
docker build -t coco/coco-aggregate-healthcheck
```

## Running

```
docker run \
    --env ETCD_PEERS=localhost:4001 \
    --env VULCAND_ADDRESS=localhost:8080 \
    --env KEY_PREFIX=/vulcand/frontends \
    --env EXCLUDE_SERVICES=aggregate-healthcheck \
    coco/coco-aggregate-healthcheck
```

When developing locally it's easier to run the binary directly, rather than through a docker container

```
go install
$GOPATH/bin/coco-aggregate-healthcheck \
    --ectd-peers localhost:4001 \
    --vulcand localhost:8080 \
    --key-prefix /vulcand/frontends \
    --exclude aggregate-healthcheck
```

You can also use an SSH tunnel as a SOCKS proxy

```
ssh -D 2323 -N core@$FLEETCTL_TUNNEL
$GOPATH/bin/coco-aggregate-healthcheck \
    --socks-proxy localhost:2323 \
    --ectd-peers localhost:4001 \
    --vulcand localhost:8080 \
    --key-prefix /vulcand/frontends \
    --exclude aggregate-healthcheck
```
