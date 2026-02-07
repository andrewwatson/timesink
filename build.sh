#!/bin/bash
set -e
go build -o timesink ./cmd/timesink
echo "Built ./timesink"
