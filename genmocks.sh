#!/bin/bash

go install github.com/matryer/moq@latest
moq -out handler_mock_test.go --stub -pkg muxter $(go list -f '{{.Dir}}' std | grep http$) Handler