#!/bin/sh
# wait-for-it.sh

host="$1"
shift
cmd="$@"

until nc -z ${host%:*} ${host#*:}; do
  echo "Waiting for $host..."
  sleep 1
done

exec $cmd
