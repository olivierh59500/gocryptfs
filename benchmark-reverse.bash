#!/bin/bash -eu

# Benchmark gocryptfs' reverse mode

cd "$(dirname "$0")"
MYNAME=$(basename "$0")

# Download /tmp/linux-3.0.tar.gz
./tests/dl-linux-tarball.bash

cd /tmp
PLAIN=linux-3.0

SIZE=0
if [[ -d $PLAIN ]]; then
	SIZE=$(du -s --apparent-size $PLAIN | cut -f1)
fi


if [[ $SIZE -ne 412334 ]] ; then
	echo "Extracting linux-3.0.tar.gz"
	rm -Rf $PLAIN
	tar xf linux-3.0.tar.gz
fi

rm -f $PLAIN/.gocryptfs.reverse.conf
gocryptfs -q -init -reverse -extpass="echo test" -scryptn=10 $PLAIN

MNT=$(mktemp -d /tmp/linux-3.0.reverse.mnt.XXX)

# Cleanup trap
trap "rm -f $PLAIN/.gocryptfs.reverse.conf ; fusermount -u -z $MNT ; rmdir $MNT" EXIT

# Mount
gocryptfs -q -reverse -extpass="echo test" $PLAIN $MNT

# Print elapsed wall time only
function etime {
	LC_ALL=C /usr/bin/time -f %e 2>&1 $@ > /dev/null
}

echo -n "LS: "
etime ls -lR $MNT

