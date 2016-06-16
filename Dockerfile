FROM alpine

ADD . /aggregate-healthcheck/

RUN apk --update add go git\
  && export GOPATH=/.gopath \
  && go get github.com/Financial-Times/aggregate-healthcheck \
  && cd $GOPATH/src/github.com/Financial-Times/aggregate-healthcheck \
  && git checkout temp \
  && cd $GOPATH/src/github.com/Financial-Times/go-fthealth \
  && git checkout ack-support \
  && cd $GOPATH/src/github.com/Financial-Times/aggregate-healthcheck \
  && go build \
  && mv aggregate-healthcheck /aggregate-healthcheck-app \
  && mv main.html /main.html \
  && apk del go git \
  && rm -rf $GOPATH /var/cache/apk/*

EXPOSE 8080

CMD [ "/aggregate-healthcheck-app" ]
