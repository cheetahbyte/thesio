# Build the Typst library once, then run the app with live reload.
start:
    scripts/build-typst.sh
    CGO_ENABLED=1 go tool mygo dev

# An optimized app bundle and disk image in build/, for the oldest macOS in mygo.json.
bundle:
    #!/bin/sh
    set -eu
    if [ "$(uname)" = Darwin ]; then
      export MACOSX_DEPLOYMENT_TARGET="$(plutil -extract macos.minimumSystemVersion raw mygo.json)"
      export CGO_CFLAGS="-O2 -g -mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET"
    fi
    scripts/build-typst.sh
    CGO_ENABLED=1 go tool mygo build

check:
    scripts/check.sh
