# TBD

### Fixes
* Adds the package.json changed by the minimal GRPC server during pre-release to staging area
* Fixes a bug where we try to add a file which doesn't exist that breaks the releae process

# 0.1.8

### Fixes
* Fixes how we add files during a release, using a white list of files

# 0.1.7

### Changes
* Migrate repo to use new release workflow
* Merge `develop` into `master`

### Fixes
* Fix `invalidDockerImgCharsRegex` in `get-docker-tag`

# 0.1.6
* Add flag to bump major version for release command

# 0.1.5
* Require authentication through personal access token for release command

# 0.1.4
* Update goreleaser config to latest version of goreleaser

# 0.1.3
* Fix goreleaser config by changing from bin.install kurtosis -> bin.install kudet

# 0.1.2
* Update circle ci configuration to not push artifacts on PR branches

# 0.1.1
* Add necessary versioning tools

# 0.1.0
* Initial commit