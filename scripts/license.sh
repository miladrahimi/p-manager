#!/bin/bash

script_dir=$(dirname "$(realpath "$0")")

if [ -z "$1" ]; then
    if [ -e "$script_dir/../storage/license.txt" ]; then
        echo "License:"
        cat "$script_dir/../storage/license.txt"
    else
        echo "No license."
    fi
else
    echo "$1" > "$script_dir/../storage/license.txt"
    echo "License updated."
fi

