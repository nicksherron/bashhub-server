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

CONTAINER="bashhub-postgres-test"

docker run -d --rm --name ${CONTAINER} -p 5444:5432 postgres


until [ "$(docker exec bashhub-postgres-test pg_isready \
          -p 5432 -h localhost -U postgres -d postgres)" == "localhost:5432 - accepting connections" ]; do
    sleep 0.1;
done;

go test -v  github.com/nicksherron/bashhub-server/internal \
  -postgres -postgres-uri "postgres://postgres:@localhost:5444?sslmode=disable"

docker stop -t 0 ${CONTAINER} & docker wait ${CONTAINER}
