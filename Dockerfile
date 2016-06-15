FROM alpine

ADD . /aggregate-healthcheck/

RUN apk --update add go git gcc linux-headers libc-dev\
  && export GOPATH=/.gopath \
  && go get github.com/Financial-Times/aggregate-healthcheck \
  && cd aggregate-healthcheck \
  && CGO_ENABLED=0 go build -a -installsuffix cgo -o aggregate-healthcheck . \
  && mv aggregate-healthcheck /aggregate-healthcheck-app \
  && mv main.html /main.html \
  && apk del go git \
  && rm -rf $GOPATH /var/cache/apk/*

EXPOSE 8080

CMD [ "/aggregate-healthcheck-app" ]
