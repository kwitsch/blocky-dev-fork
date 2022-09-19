#!/bin/sh

echo "::group::Environment - ${TARGETARCH}${TARGETVARIANT}"
echo "CC: ${go env CC}"
echo "CXX: ${go env CXX}"
echo "CGO_ENABLED: ${go env CGO_ENABLED}"
echo "GOOS: ${go env GOOS}"
echo "GOARCH: ${go env GOARCH}"
echo "GOARM: ${go env GOARM}"
echo "::endgroup::"