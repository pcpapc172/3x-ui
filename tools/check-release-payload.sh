#!/usr/bin/env bash
set -euo pipefail

payload=${1:?usage: check-linux-payload.sh PAYLOAD_DIR ARCH VERSION}
arch=${2:?missing architecture}
version=${3:?missing version}

for name in x-ui install.sh update.sh x-ui.sh x-ui.rc x-ui.service.debian x-ui.service.arch x-ui.service.rhel; do
    [[ -s "$payload/$name" ]] || { echo "Missing release payload: $name" >&2; exit 1; }
done
for name in "xray-linux-$arch" "mtg-linux-$arch"; do
    [[ -x "$payload/bin/$name" ]] || { echo "Missing executable: bin/$name" >&2; exit 1; }
done
for name in geoip.dat geosite.dat geoip_IR.dat geosite_IR.dat geoip_RU.dat geosite_RU.dat; do
    [[ -s "$payload/bin/$name" ]] || { echo "Missing geodata: bin/$name" >&2; exit 1; }
done
bash -n "$payload/install.sh" "$payload/update.sh" "$payload/x-ui.sh"
sh -n "$payload/x-ui.rc"
[[ -x "$payload/x-ui" ]] || { echo "Panel binary is not executable" >&2; exit 1; }
if [[ "$arch" == amd64 && "$(uname -m)" == x86_64 ]]; then
    actual=$("$payload/x-ui" -v)
    [[ "$actual" == "$version" ]] || { echo "Panel version $actual does not match release $version" >&2; exit 1; }
fi
printf 'Release payload verified: %s (%s)\n' "$version" "$arch"
