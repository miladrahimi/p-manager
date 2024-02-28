#!/bin/bash

script_dir=$(dirname "$(realpath "$0")")
license_path="$script_dir/../storage/license.txt"

if [ -z "$1" ]; then
    if [ -e "$license_path" ]; then
        echo "License:"
        cat "$license_path"
    else
        echo "No license."
    fi
else
    echo "$1" > "$license_path"
    echo "License updated."
fi

