#!/bin/bash
pkill -9 '^(harmony|soldier|commander|profiler|bootnode)$' | sed 's/^/Killed process: /'
rm -rf db-127.0.0.1-*
# 同时删除任何serverIP构建的目录
rm -rf db-10.20.6.199-*
rm -rf db-10.20.82.234-*
rm -rf db-10.20.0.57-*
#rm -rf db-10.20.3.46-*
rm -rf db-10.20.36.229-*