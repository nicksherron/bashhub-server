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

version="$(git tag | sort --version-sort -r | head -1)"

last_tag_ref=$(git log --decorate | grep $version | cut -d' ' -f 2 | head -c 7)

last_commit_ref="$(git log -1 --oneline | cut -d' ' -f 1)"

echo $version $last_tag_ref $last_commit_ref

if [ ! $last_tag_ref == $last_commit_ref ]; then
	echo "last commit is not last tag"
	echo "last tag: $version"
	exit 1
fi

export GITHUB_USER=nicksherorn
export GITHUB_REPO=bashhub-server

echo "Verifying release"
if hub release | grep $version >/dev/null 2>&1; then
	echo "    This version already exists."
	exit 0
fi

echo "Creating release $version"
git push origin $version

ghr $version dist/
