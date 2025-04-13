#!/bin/bash
nohup ./abyss_open_reg &
pid=$!
echo "kill $pid" > kill.sh
chmod +x kill.sh