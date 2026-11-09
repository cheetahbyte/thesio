#!/bin/sh
# Fails when an executable in the app bundle needs a newer macOS than the bundle's
# LSMinimumSystemVersion: Finder then marks the app as one that cannot be opened.
#   scripts/check-macos-target.sh build/Thesio.app
set -eu
app=$1
min=$(plutil -extract LSMinimumSystemVersion raw "$app/Contents/Info.plist")
status=0
checked=0
for bin in "$app"/Contents/MacOS/*; do
  if ! archs=$(lipo -archs "$bin") || [ -z "$archs" ]; then
    echo "$bin: not a Mach-O executable"
    status=1
    continue
  fi
  checked=1
  for arch in $archs; do
    otool -arch "$arch" -l "$bin" | awk -v min="$min" -v where="$bin ($arch)" '
      function newer(a, b,   x, y, i) {
        split(a, x, "."); split(b, y, ".")
        for (i = 1; i <= 3; i++) if (x[i] + 0 != y[i] + 0) return x[i] + 0 > y[i] + 0
        return 0
      }
      $1 == "minos" { found = 1; if (newer($2, min)) bad = $2 }
      END {
        if (!found) { print where ": no minos in LC_BUILD_VERSION"; exit 1 }
        if (bad != "") { print where ": needs macOS " bad ", Info.plist says " min; exit 1 }
        print where ": macOS " min
      }' || status=1
  done
done
[ $checked = 1 ] || { echo "$app: no Mach-O executable in Contents/MacOS"; status=1; }
exit $status
