#!/bin/bash
set -ex

oc delete validatingwebhookconfiguration/vswift.kb.io --ignore-not-found
oc delete mutatingwebhookconfiguration/mswift.kb.io --ignore-not-found
