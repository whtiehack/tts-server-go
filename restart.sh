set -e

go build -o ttl-server ./cmd/cli

pkill -f ttl-server || true
mv log.txt log_back.txt || true
nohup ./ttl-server > log.txt 2>&1 &