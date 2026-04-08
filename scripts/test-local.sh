#!/usr/bin/env bash
set -euo pipefail

# Comma-separated import paths to skip for local policy-constrained environments.
GO_TEST_EXCLUDE="${GO_TEST_EXCLUDE:-rewrite/internal/modules/webhooks}"

mapfile -t all_pkgs < <(go list ./...)
filtered=()

for pkg in "${all_pkgs[@]}"; do
  skip=false
  IFS=',' read -r -a excluded <<< "${GO_TEST_EXCLUDE}"
  for ex in "${excluded[@]}"; do
    ex_trimmed="$(echo "${ex}" | xargs)"
    if [[ -n "${ex_trimmed}" && "${pkg}" == "${ex_trimmed}" ]]; then
      skip=true
      break
    fi
  done

  if [[ "${skip}" == false ]]; then
    filtered+=("${pkg}")
  fi
done

if [[ "${#filtered[@]}" -eq 0 ]]; then
  echo "no packages left to test after applying GO_TEST_EXCLUDE"
  exit 1
fi

go test "${filtered[@]}"
