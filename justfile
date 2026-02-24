#!/usr/bin/env -S just --justfile

set windows-shell := ["powershell.exe", "-NoLogo", "-Command"]
set shell := ["bash", "-cu"]

ready:
  just fmt
  just test

[unix]
init:
  git submodule update --init --depth 1 typescript-go
  @if ls patches/*.patch 1>/dev/null 2>&1; then \
    git -C typescript-go config user.email "ci@tsgonest"; \
    git -C typescript-go config user.name  "tsgonest CI"; \
    pushd typescript-go && git am --3way --no-gpg-sign ../patches/*.patch && popd; \
  fi
  mkdir -p internal/collections && find ./typescript-go/internal/collections -type f ! -name '*_test.go' -exec cp {} internal/collections/ \;

[windows]
init:
  git submodule update --init --depth 1 typescript-go
  git -C typescript-go config user.email "ci@tsgonest"
  git -C typescript-go config user.name  "tsgonest CI"
  pushd typescript-go; Get-ChildItem ../patches/*.patch -ErrorAction SilentlyContinue | ForEach-Object { git am --3way --no-gpg-sign $_.FullName }; popd
  New-Item -ItemType Directory -Force -Path internal\collections
  Get-ChildItem -Path .\typescript-go\internal\collections\* -File | Where-Object { $_.Name -notlike '*_test.go' } | ForEach-Object { Copy-Item $_.FullName -Destination .\internal\collections\ }

[unix]
build:
  go build -o tsgonest ./cmd/tsgonest
  @mkdir -p packages/core/bin
  @cp tsgonest packages/core/bin/tsgonest
  @chmod 755 packages/core/bin/tsgonest

[windows]
build:
  $env:GOOS="windows"; $env:GOARCH="amd64"; go build -o tsgonest.exe ./cmd/tsgonest
  New-Item -ItemType Directory -Force -Path packages\core\bin
  Copy-Item tsgonest.exe packages\core\bin\tsgonest.exe

test: build
  go test ./internal/...
  cd e2e && pnpm run test --run && cd ..

test-unit:
  go test ./internal/...

test-e2e: build
  cd e2e && pnpm run test --run && cd ..

fmt:
  gofmt -w internal cmd tools

shim:
  go run tools/gen_shims/main.go

bench: build
  cd benchmarks && pnpm run build && pnpm run bench:all

bench-json: build
  cd benchmarks && pnpm run build && pnpm run bench:json

clean:
  rm -f tsgonest tsgonest.exe
  rm -rf dist/
