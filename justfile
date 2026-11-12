#!/usr/bin/env -S just --justfile

set windows-shell := ["powershell.exe", "-NoLogo", "-Command"]
set shell := ["bash", "-cu"]

ready:
  just fmt
  just test

[unix]
init:
  git submodule update --init --depth 1 typescript
  @if ls patches/*.patch 1>/dev/null 2>&1; then \
    git -C typescript config user.email "ci@tsgonest"; \
    git -C typescript config user.name  "tsgonest CI"; \
    pushd typescript && git am --3way --no-gpg-sign ../patches/*.patch && popd; \
  fi

[windows]
init:
  git submodule update --init --depth 1 typescript
  git -C typescript config user.email "ci@tsgonest"
  git -C typescript config user.name  "tsgonest CI"
  if ((Get-ChildItem patches/*.patch -ErrorAction SilentlyContinue).Count -gt 0) { Push-Location typescript; Get-ChildItem ../patches/*.patch | ForEach-Object { git am --3way --no-gpg-sign $_.FullName; if ($LASTEXITCODE -ne 0) { throw "git am failed for $($_.Name)" } }; Pop-Location }

[unix]
build:
  go build -o tsgonest ./cmd/tsgonest
  @mkdir -p packages/core/bin
  @cp tsgonest packages/core/bin/tsgonest-native
  @chmod 755 packages/core/bin/tsgonest-native

[windows]
build:
  go build -o tsgonest.exe ./cmd/tsgonest
  New-Item -ItemType Directory -Force -Path packages\core\bin | Out-Null
  Copy-Item tsgonest.exe packages\core\bin\tsgonest.exe

test: build
  go test -race ./internal/...
  pnpm --dir e2e run test --run

test-unit:
  go test -race ./internal/...

test-e2e: build
  pnpm --dir e2e run test --run

fmt:
  gofmt -w internal cmd tools

shim:
  go run tools/gen_shims/main.go

bench: build
  cd benchmarks && pnpm run build && pnpm run bench:all

bench-json: build
  cd benchmarks && pnpm run build && pnpm run bench:json

[unix]
clean:
  rm -f tsgonest tsgonest.exe
  rm -rf dist/

[windows]
clean:
  Remove-Item -Force -ErrorAction SilentlyContinue tsgonest, tsgonest.exe
  Remove-Item -Force -Recurse -ErrorAction SilentlyContinue dist
