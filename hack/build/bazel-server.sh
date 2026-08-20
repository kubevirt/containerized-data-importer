curl -L -o /usr/bin/bazel https://github.com/bazelbuild/bazel/releases/download/6.5.0/bazel-6.5.0-linux-x86_64
chmod 777 /usr/bin/bazel
sleep 10
which bazel
BAZEL_PID=$(bazel info | grep server_pid | cut -d " " -f 2)
while kill -0 $BAZEL_PID 2>/dev/null; do sleep 1; done
# Might not be necessary, just to be sure that exec shutdowns always succeed
# and are not killed by docker.
sleep 1
