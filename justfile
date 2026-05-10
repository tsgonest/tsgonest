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
  just _patch-collections

# Rewrites typescript-go's internal/json wrapper imports back to the upstream
# go-json-experiment packages, since we can't import typescript-go internals.
[unix]
_patch-collections:
  @if grep -q '"github.com/microsoft/typescript-go/internal/json"' internal/collections/ordered_map.go 2>/dev/null; then \
    sed -i.bak \
      -e 's|"github.com/microsoft/typescript-go/internal/json"|"github.com/go-json-experiment/json"\n\t"github.com/go-json-experiment/json/jsontext"|' \
      -e 's|enc \*json\.Encoder|enc *jsontext.Encoder|g' \
      -e 's|dec \*json\.Decoder|dec *jsontext.Decoder|g' \
      -e 's|json\.BeginObject|jsontext.BeginObject|g' \
      -e 's|json\.EndObject|jsontext.EndObject|g' \
      internal/collections/ordered_map.go && \
    rm -f internal/collections/ordered_map.go.bak; \
  fi

[windows]
init:
  git submodule update --init --depth 1 typescript-go
  git -C typescript-go config user.email "ci@tsgonest"
  git -C typescript-go config user.name  "tsgonest CI"
  if ((Get-ChildItem patches/*.patch -ErrorAction SilentlyContinue).Count -gt 0) { Push-Location typescript-go; Get-ChildItem ../patches/*.patch | ForEach-Object { git am --3way --no-gpg-sign $_.FullName; if ($LASTEXITCODE -ne 0) { throw "git am failed for $($_.Name)" } }; Pop-Location }
  New-Item -ItemType Directory -Force -Path internal\collections | Out-Null
  Get-ChildItem -Path .\typescript-go\internal\collections\* -File | Where-Object { $_.Name -notlike '*_test.go' } | ForEach-Object { Copy-Item $_.FullName -Destination .\internal\collections\ }
  just _patch-collections

[windows]
_patch-collections:
  $f = "internal\collections\ordered_map.go"; if (Test-Path $f) { $c = Get-Content -Raw $f; if ($c -match '"github.com/microsoft/typescript-go/internal/json"') { $c = $c -replace '"github.com/microsoft/typescript-go/internal/json"', "`"github.com/go-json-experiment/json`"`r`n`t`"github.com/go-json-experiment/json/jsontext`""; $c = $c -replace 'enc \*json\.Encoder', 'enc *jsontext.Encoder'; $c = $c -replace 'dec \*json\.Decoder', 'dec *jsontext.Decoder'; $c = $c -replace 'json\.BeginObject', 'jsontext.BeginObject'; $c = $c -replace 'json\.EndObject', 'jsontext.EndObject'; Set-Content -NoNewline $f $c } }

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

[unix]
clean:
  rm -f tsgonest tsgonest.exe
  rm -rf dist/

[windows]
clean:
  Remove-Item -Force -ErrorAction SilentlyContinue tsgonest, tsgonest.exe
  Remove-Item -Force -Recurse -ErrorAction SilentlyContinue dist
