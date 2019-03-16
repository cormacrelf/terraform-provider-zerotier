#!/usr/bin/env bash

# Check gofmt
echo "==> Checking that code complies with gofmt requirements..."
gofmt_files=$(gofmt -l $(find . -name '*.go' | grep -v vendor))
if [[ -n ${gofmt_files} ]]; then
    echo 'gofmt needs running on the following files:'
    echo "${gofmt_files}"
    echo "You can use the command: \`make fmt\` to reformat code."
    exit 1
fi

if ! which goimports >/dev/null; then
    echo "==> Installing goimports..."
    go get golang.org/x/tools/cmd/goimports
fi

exit 0
