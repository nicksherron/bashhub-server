# Build Stage
FROM nicksherron/bashhub-server-build:1.13 AS build-stage

LABEL app="build-bashhub-server"
LABEL REPO="https://github.com/nicksherron/bashhub-server"

ENV PROJPATH=/go/src/github.com/nicksherron/bashhub-server

# Because of https://github.com/docker/docker/issues/14914
ENV PATH=$PATH:$GOROOT/bin:$GOPATH/bin

ADD . /go/src/github.com/nicksherron/bashhub-server
WORKDIR /go/src/github.com/nicksherron/bashhub-server

RUN make build-alpine

# Final Stage
FROM nicksherron/bashhub-server

ARG GIT_COMMIT
ARG VERSION
LABEL REPO="https://github.com/nicksherron/bashhub-server"
LABEL GIT_COMMIT=$GIT_COMMIT
LABEL VERSION=$VERSION

# Because of https://github.com/docker/docker/issues/14914
ENV PATH=$PATH:/opt/bashhub-server/bin

WORKDIR /opt/bashhub-server/bin

COPY --from=build-stage /go/src/github.com/nicksherron/bashhub-server/bin/bashhub-server /opt/bashhub-server/bin/
RUN chmod +x /opt/bashhub-server/bin/bashhub-server

# Create appuser
RUN adduser -D -g '' bashhub-server
USER bashhub-server

ENTRYPOINT ["/usr/bin/dumb-init", "--"]

CMD ["/opt/bashhub-server/bin/bashhub-server"]
