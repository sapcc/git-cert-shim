#!/bin/sh
CERT=$1
shift
ssh-agent -k sh -c "ssh-add $CERT > /dev/null; git -C $*"
