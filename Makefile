TEST_SERVER=
TEST_USER=ec2-user
JUMPHOST=

linux_binary:
	GOOS=linux go build -o linux_binary github.com/devsbb/jenkins-spot-instance-terminator/cmd

copy_linux_binary: linux_binary
	rsync -e "ssh -J ${TEST_USER}@${JUMPHOST} -v" -av --compress --progress linux_binary ${TEST_USER}@${TEST_SERVER}:~/

run_on_server: copy_linux_binary
	ssh -J ${TEST_USER}@${JUMPHOST} ${TEST_USER}@${TEST_SERVER} ./linux_binary

.PHONY=copy_linux_binary run_on_server
