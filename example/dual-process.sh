#!/usr/bin/env bash

echo "Starting Process 01"
nohup python /main.py >process1.log &
echo "Running Process 01"



echo "Starting Process 02"
python /main.py