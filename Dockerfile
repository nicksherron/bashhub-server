# Copyright © 2020 nicksherron <nsherron90@gmail.com>
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
#

# GitHub:       https://github.com/nicksherron/bashhub-server
FROM golang:1.13-alpine AS build

ARG VERSION
ARG GIT_COMMIT
ARG BUILD_DATE

ARG CGO=1
ENV CGO_ENABLED=${CGO}
ENV GOOS=linux
ENV GO111MODULE=on

WORKDIR /go/src/github.com/nicksherron/bashhub-server

COPY . /go/src/github.com/nicksherron/bashhub-server/

# gcc/g++ are required to build SASS libraries for extended version
RUN apk update && \
    apk add --no-cache gcc g++ musl-dev


RUN go build  -ldflags "-X github.com/nicksherron/bashhub-server/cmd.Version=${VERSION} -X github.com/nicksherron/bashhub-server/cmd.GitCommit=${GIT_COMMIT} -X github.com/nicksherron/bashhub-server/cmd.BuildDate=${BUILD_DATE}" -o /go/bin/bashhub-server

# ---

FROM alpine:3.11

COPY --from=build /go/bin/bashhub-server /usr/bin/bashhub-server

# libc6-compat & libstdc++ are required for extended SASS libraries
# ca-certificates are required to fetch outside resources (like Twitter oEmbeds)
RUN apk update && \
    apk add --no-cache ca-certificates libc6-compat libstdc++

VOLUME /data
WORKDIR /data

# Expose port for live server
EXPOSE 8080

ENTRYPOINT ["bashhub-server"]
CMD [ "--db", "./data.db"]
