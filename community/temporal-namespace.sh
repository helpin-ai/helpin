#!/bin/sh
set -eu
temporal operator cluster health --address temporal:7233 >/dev/null
if ! temporal operator namespace describe --address temporal:7233 --namespace default >/dev/null 2>&1; then
  temporal operator namespace create --address temporal:7233 --namespace default --retention 72h
fi
