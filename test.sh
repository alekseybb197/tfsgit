#!/bin/bash

make build

. ../.env
export tfspath=...some_path_like...project/subproject/dev/dir...

test -d test || mkdir test
cd test
../dist/tfsgit

