#!/usr/bin/env bash

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
export GITHUB_REPO=proxi

echo "Verifying release"
if hub release | grep $version >/dev/null 2>&1; then
	echo "    This version already exists."
	exit 0
fi

echo "Creating release $version"
git push origin $version

ghr $version dist/
