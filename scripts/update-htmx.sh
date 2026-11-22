#!/usr/bin/env bash
# Replaces the embedded htmx file with another version from the npm registry.
# Usage: scripts/update-htmx.sh <version>      for example: scripts/update-htmx.sh 2.0.11
#
# The script checks the npm "integrity" hash (SHA-512) of the package before it uses the file.
# It updates internal/web/static/htmx-<version>.min.js, internal/web/vendor.txt,
# and the <script> tag in internal/web/views.templ. Then it runs templ generate.
set -euo pipefail
cd "$(dirname "$0")/.."

version=${1:?usage: scripts/update-htmx.sh <version>}
[[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "invalid version: $version" >&2; exit 1; }

static=internal/web/static
out="$static/htmx-$version.min.js"

python3 -I - "$version" "$out" <<'PY'
import base64, hashlib, io, json, sys, tarfile, urllib.request

version, out = sys.argv[1], sys.argv[2]
with urllib.request.urlopen(f"https://registry.npmjs.org/htmx.org/{version}") as r:
    dist = json.load(r)["dist"]
algo, _, expected = dist["integrity"].partition("-")
if algo != "sha512":
    sys.exit(f"unexpected integrity algorithm: {algo}")
with urllib.request.urlopen(dist["tarball"]) as r:
    tarball = r.read()
if base64.b64encode(hashlib.sha512(tarball).digest()).decode() != expected:
    sys.exit("the tarball does not match the npm integrity hash")
with tarfile.open(fileobj=io.BytesIO(tarball), mode="r:gz") as tar:
    data = tar.extractfile("package/dist/htmx.min.js").read()
with open(out, "wb") as f:
    f.write(data)
PY

for old in "$static"/htmx-*.min.js "$static"/htmx.min.js; do
  [[ -e $old && $old != "$out" ]] && rm "$old"
done
sed -i.bak -E "s#/static/htmx(-[0-9.]+)?\.min\.js#/static/htmx-$version.min.js#" internal/web/views.templ
rm internal/web/views.templ.bak

sha=$(python3 -I -c 'import hashlib,sys; print(hashlib.sha256(open(sys.argv[1],"rb").read()).hexdigest())' "$out")
cat > internal/web/vendor.txt <<TXT
# Third-party files in internal/web/static. Update them with the scripts in scripts/.
# name    version   sha256                                                            source
htmx      $version   $sha  https://registry.npmjs.org/htmx.org/$version
TXT

go tool templ generate >/dev/null
echo "htmx $version: $out ($sha)"
