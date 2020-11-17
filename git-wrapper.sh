#!/bin/sh
CERT=$1
shift
ssh-agent sh -c "ssh-add $CERT > /dev/null; git -C $*"
