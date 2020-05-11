TEST_SERVER=
TEST_USER=ec2-user
JUMPHOST=

development-checks: lint test

lint:
	golangci-lint run

test:
	go test ./...

linux:
	GOOS=linux GOARCH=amd64 go build \
	-o bin/jenkins-spot-instance-terminator-linux-amd64 \
	github.com/devsbb/jenkins-spot-instance-terminator/cmd

darwin:
	GOOS=darwin GOARCH=amd64 go build \
	-o bin/jenkins-spot-instance-terminator-darwin-amd64 \
	github.com/devsbb/jenkins-spot-instance-terminator/cmd

copy_linux_binary: linux
	rsync \
	-e "ssh -J ${TEST_USER}@${JUMPHOST} -v" \
	-av --compress --progress \
	bin/jenkins-spot-instance-terminator-linux-amd64 \
	${TEST_USER}@${TEST_SERVER}:~/

run_on_server: copy_linux_binary
	ssh -J ${TEST_USER}@${JUMPHOST} ${TEST_USER}@${TEST_SERVER} ./jenkins-spot-instance-terminator-linux-amd64

.PHONY=copy_linux_binary run_on_server linux darwin development-checks lint test

