#!/bin/bash -e

: ${1?= required} # pod name
: ${2?= required} # path to write to
: ${3?= required} # data to write

echo "Writing data ($3) to $1..."
/usr/local/bin/kubectl exec -i $1 -- bash -c "cat << EOF > $2
$3
EOF"
