#!/usr/bin/env bash

#TODO:issue temporary solution, see https://github.com/octomation/go-module/issues/106
init() {
  local from to=${1}
  from=$(go list -m)

  make tools
  go mod edit -module "${to}"
  find . -type file -name '*.go' -exec sed -i '' "s|${from}|${to}|g" {} +
  make format
}
