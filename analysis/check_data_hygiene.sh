#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

fail=0

if git ls-files | grep -Eq '(^|/)(Датасет_Рекомендации|.*Датасет_Рекомендации).*\.(xlsx|xls|csv)$'; then
  echo "FAIL: raw competition dataset is tracked"
  fail=1
fi

if git grep -nE '\bC[0-9]{5}\b|\bO[0-9]{5}\b' -- ':!*.md' ':!*.json' ':!*.csv' >/dev/null 2>&1; then
  echo "WARN: real-looking client/order ids in tracked text"
  git grep -nE '\bC[0-9]{5}\b|\bO[0-9]{5}\b' -- ':!*.md' ':!*.json' ':!*.csv' | head -10
  fail=1
fi

for f in $(git ls-files '*.csv'); do
  if grep -Eq '\bC[0-9]{5}\b|\bO[0-9]{5}\b' "$f"; then
    echo "FAIL: real-looking ids in $f"
    fail=1
  fi
done

if [ "$fail" -eq 0 ]; then
  echo "OK: no raw dataset or real client/order ids in tracked files"
fi
exit "$fail"
