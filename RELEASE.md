# Release Process

depstat is released on an as-needed basis. The process is as follows:

1. An issue proposing a new release with a changelog since the last release is created.
2. All [OWNERS](OWNERS) must LGTM this release.
3. An OWNER tags a particular commit that needs to be released using the command - `$ git tag <version-number> <git-commit>`. For example `git tag v0.7.0-rc.2 a56be6f877623913c7322becd29489397203364d`. This commit has to be part of the `main` branch
4. An OWNER pushes the git tag using `$ git push <git-remote-ref> <tag>`. For example `git push origin v0.7.0-rc.2`
5. On pushing the git tag, the GitHub Actions [release](https://github.com/kubernetes-sigs/depstat/blob/main/.github/workflows/release.yml) workflow is automatically triggered. The workflow runs [`goreleaser`](https://goreleaser.com/) command to automatically release `depstat` by doing the following automatically:
    - Building the latest binaries for various platforms (OSes) and architectures and pack them in tar balls
    - Create a checksums.txt for the tar balls
    - Find changelog of commits between previous and current release
    - Create a GitHub release
    - Upload the tar balls and tar balls to the GitHub release
    - Add changelog of commits to the GitHub release description
6. The release issue is closed.
7. An announcement email is sent to `kubernetes-dev@googlegroups.com` with the subject `[ANNOUNCE] depstat $VERSION is released`.
