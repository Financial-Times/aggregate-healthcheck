FROM golang:1.7-alpine3.5

RUN mkdir -p /aggregate-healthcheck

ADD . "$GOPATH/src/aggregate-healthcheck"

RUN apk --no-cache --virtual .build-dependencies add git \
  && cd $GOPATH/src/aggregate-healthcheck \
  && go-wrapper download \
  && go-wrapper install \
  && pwd \
  && ls -la $GOPATH \
  && cp main.html /aggregate-healthcheck/ \
  && cp -R resources /aggregate-healthcheck/ \
  && apk del .build-dependencies \
  && rm -rf $GOPATH/src $GOPATH/pkg

WORKDIR /aggregate-healthcheck

EXPOSE 8080

CMD ["go-wrapper", "run"]