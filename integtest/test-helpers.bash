#!/usr/bin/env bash

_assert_string_equal() {
  v="$1"
  expected="$2"

  if [[ "$v" != "$expected" ]]; then
      echo "error: got \"$v\", expected \"$expected\""
#      exit 1
  fi
}

_assert_string_match() {
  v="$1"
  pattern="$2"

  if [[ ! "$v" =~ $pattern ]]; then
      echo "error: \"$v\" doesn't match /$pattern/"
#      exit 1
  fi
}

_until_success() {
    declare command="$1"
    timeout --preserve-status 600s bash -c "until $command > /dev/null 2>&1; do sleep 5; done"
}