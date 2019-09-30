#!/usr/bin/env bash

go build
cd client
go build
cd ..

for i in `seq 1 3`;
do
    peerPort = i + 5000
    peer="127.0.0.1:$peerPort"
    gossipAddr="127.0.0.1:$gossipPort"
done