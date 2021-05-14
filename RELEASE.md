# Release Process

depstat is released on an as-needed basis. The process is as follows:

1. An issue proposing a new release with a changelog since the last release is created.
2. All [OWNERS](OWNERS) must LGTM this release.
3. An OWNER builds the latest binary with the appropriate tag by running `go build -ldflags "-X main.DepstatVersion=<version-number>"`.
4. An OWNER uses the GitHub releases page to create a new release and drops the built binaries along with the changelog. 
5. The release issue is closed.
6. An announcement email is sent to `kubernetes-dev@googlegroups.com` with the subject `[ANNOUNCE] depstat $VERSION is released`.
