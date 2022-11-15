#!/bin/bash

set -e
cd $(cd "$(dirname $0)";pwd)

GOOS=linux go build -o tts-server ./cmd/cli

ssh -t root@192.168.1.10 "rm -rf /home/qianqiu/download/tts-server"
scp -r ./tts-server root@192.168.1.10:/home/qianqiu/download/
ssh -t root@192.168.1.10 "chmod +x /home/qianqiu/download/tts-server && pm2 restart tts-server"
