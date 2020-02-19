#!/usr/bin/env bash

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

set -eou pipefail

CONTAINER_1="bashhub-postgres-test"

docker run -d --rm --name ${CONTAINER_1} -p 5444:5432 postgres


until [ "$(docker exec ${CONTAINER_1} pg_isready \
          -p 5432 -h localhost -U postgres -d postgres)" == "localhost:5432 - accepting connections" ]; do
    sleep 0.1;
done;

go test github.com/nicksherron/bashhub-server/internal \
  -postgres-uri "postgres://postgres:@localhost:5444?sslmode=disable"

docker stop -t 0 ${CONTAINER_1} & docker wait ${CONTAINER_1}


CONTAINER_2="bashhub-postgres-test-1"
CONTAINER_3="bashhub-postgres-test-2"


docker run -d --rm --name ${CONTAINER_2} -p 5445:5432 postgres
docker run -d --rm --name ${CONTAINER_3} -p 5446:5432 postgres

until [ "$(docker exec ${CONTAINER_2} pg_isready \
          -p 5432 -h localhost -U postgres -d postgres)" == "localhost:5432 - accepting connections" ]; do
    sleep 0.1;
done;


until [ "$(docker exec ${CONTAINER_3} pg_isready \
          -p 5432 -h localhost -U postgres -d postgres)" == "localhost:5432 - accepting connections" ]; do
    sleep 0.1;
done;


go test github.com/nicksherron/bashhub-server/cmd \
 -src-postgres-uri "postgres://postgres:@localhost:5445?sslmode=disable" \
 -dst-postgres-uri "postgres://postgres:@localhost:5446?sslmode=disable"

docker stop -t 0 ${CONTAINER_2} & docker wait ${CONTAINER_2}
docker stop -t 0 ${CONTAINER_3} & docker wait ${CONTAINER_3}