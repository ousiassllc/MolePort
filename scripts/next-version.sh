#!/usr/bin/env bash
# 直近の v*.*.* タグから <ref> (既定 HEAD) までのコミットを解析し、次のバージョンを標準出力に表示する。
# リリース対象のコミットがない、または <ref> に既にバージョンタグがある場合は何も出力しない。
#
# 判定ルール (マージコミットは対象外):
#   major: `type!:` 形式の件名、または本文に `BREAKING CHANGE:`
#   minor: `feat:`
#   patch: `fix:` / `update:` / `refactor:` / `perf:`
#   上記以外 (docs / test / ci など) のみ: リリースしない
set -euo pipefail

ref="${1:-HEAD}"
tag_glob='v[0-9]*.[0-9]*.[0-9]*'

if [ -n "$(git tag --points-at "$ref" --list "$tag_glob")" ]; then
  exit 0
fi

last="$(git describe --tags --abbrev=0 --match "$tag_glob" "$ref" 2>/dev/null || true)"
range="$ref"
[ -n "$last" ] && range="${last}..${ref}"

subjects="$(git log --no-merges --format=%s "$range")"
bodies="$(git log --no-merges --format=%b "$range")"

if grep -qE '^[a-z]+(\([^)]*\))?!:' <<<"$subjects" || grep -qE '^BREAKING[ -]CHANGE:' <<<"$bodies"; then
  bump=major
elif grep -qE '^feat(\([^)]*\))?:' <<<"$subjects"; then
  bump=minor
elif grep -qE '^(fix|update|refactor|perf)(\([^)]*\))?:' <<<"$subjects"; then
  bump=patch
else
  exit 0
fi

IFS=. read -r major minor patch <<<"${last#v}"
major="${major:-0}" minor="${minor:-0}" patch="${patch:-0}"

case "$bump" in
  major) major=$((major + 1)) minor=0 patch=0 ;;
  minor) minor=$((minor + 1)) patch=0 ;;
  patch) patch=$((patch + 1)) ;;
esac

echo "v${major}.${minor}.${patch}"
