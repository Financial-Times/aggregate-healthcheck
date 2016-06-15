FROM alpine

ADD . /aggregate-healthcheck/

RUN apk --update add go git gcc linux-headers libc-dev\
  && export GOPATH=/.gopath \
  && go get github.com/Financial-Times/aggregate-healthcheck \
  && cd $GOPATH/src/github.com/Financial-Times/aggregate-healthcheck \
  && git checkout temp \
  && cd $GOPATH/src/github.com/Financial-Times/go-fthealth \
  && git checkout ack-support \
  && cd $GOPATH/src/github.com/Financial-Times/aggregate-healthcheck \
  && CGO_ENABLED=0 go build -a -installsuffix cgo -o aggregate-healthcheck . \
  && mv aggregate-healthcheck /aggregate-healthcheck-app \
  && apk del go git \
  && rm -rf $GOPATH /var/cache/apk/*

EXPOSE 8080

CMD [ "/aggregate-healthcheck-app" ]
