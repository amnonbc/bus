
set -x
scp *.go pi2:bus/
ssh pi2  "cd bus; /usr/local/go/bin/go build"
ssh pi2  "sudo systemctl restart bus"

