#!/usr/bin/env bash
# Tag only. Container compilation and publication belong to release.yml.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

if [[ -n "$(git status --porcelain)" ]]; then
  echo 'Working tree is not clean; commit your release changes first.' >&2
  git status --short
  exit 1
fi
if ! upstream=$(git rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null); then
  echo 'The branch has no upstream; push it first.' >&2
  exit 1
fi
git fetch --quiet --prune --tags
read -r behind ahead <<< "$(git rev-list --left-right --count "$upstream...HEAD")"
if [[ "$ahead" != 0 || "$behind" != 0 ]]; then
  echo "Branch differs from $upstream ($ahead ahead, $behind behind); synchronize first." >&2
  exit 1
fi

current=$(git tag -l 'v[0-9]*.[0-9]*.[0-9]*' | sed -nE 's/^v([0-9]+\.[0-9]+\.[0-9]+)$/\1/p' | sort -V | tail -1)
current=${current:-0.0.0}
version=${1:-}
if [[ -z "$version" ]]; then
  IFS=. read -r major minor patch <<< "$current"
  printf 'Latest stable tag: v%s\n  1) major: v%s.0.0\n  2) minor: v%s.%s.0\n  3) patch: v%s.%s.%s\n' \
    "$current" "$((major + 1))" "$major" "$((minor + 1))" "$major" "$minor" "$((patch + 1))"
  read -r -p '> ' choice
  case "$choice" in
    1) version="$((major + 1)).0.0" ;;
    2) version="$major.$((minor + 1)).0" ;;
    3) version="$major.$minor.$((patch + 1))" ;;
    *) echo 'Cancelled.'; exit 0 ;;
  esac
fi
version=${version#v}
if [[ ! "$version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
  echo 'Use a stable version such as 0.1.0.' >&2
  exit 1
fi
tag="v$version"
if git rev-parse --verify --quiet "refs/tags/$tag" >/dev/null; then
  echo "$tag already exists; published tags are never overwritten." >&2
  exit 1
fi
if [[ "$(printf '%s\n' "$current" "$version" | sort -V | tail -1)" != "$version" || "$version" == "$current" ]]; then
  echo "The version must be newer than $current." >&2
  exit 1
fi
remote=$(git config "branch.$(git branch --show-current).remote")
printf 'Create %s at %s and push to %s.\n' "$tag" "$(git rev-parse --short HEAD)" "$remote"
read -r -p "Type 'yes' to proceed: " answer
[[ "$answer" == yes ]] || { echo 'Cancelled.'; exit 0; }
git tag -a "$tag" -m "$tag"
git push "$remote" "refs/tags/$tag"
echo "Pushed $tag; GitHub Actions will build and publish the container."
