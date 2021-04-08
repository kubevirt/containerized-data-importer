# Version and Release

* [Overview](#overview)
* [Version Scheme](#version-scheme)
* [Releasing a New Version](#releasing-a-new-version)
    * [Promoting the Release](#promoting-the-release)
    * [PROW CI](#prow-ci)
# Overview

## Version Scheme

CDI adheres to the [semantic version definitions](https://semver.org/) format of vMAJOR.MINOR.PATCH.  These are defined as follows:

- Major - Non-backwards compatible, API contract changes.  Incrementing a Major version means the consumer will have to make changes to the way they interact with the CDI API.  Failing do to so will result in unexpected behavior.  When these changes occur, the Major version will be incremented at the end of the sprint instead of the Minor Version.

- Minor - End of Sprint release. Encapsulates non-API-breaking changes within the current Major version.  The current Sprint cycle is 2 weeks long, producing in bug fixes and feature additions.  Publishing a Minor version at the end of the cycle allows consumers to immediately access the end product of that Sprint's goals. Issues or bugs can be reported and addressed in the following Sprint.  It is expected that this patch contain myriad commits.

- Patch - mid-Sprint release for fixing blocker bugs. In the case that a bug is blocking CDI consumers' workflow, a fix may be released as soon as it is merged.  A Patch should be limited expressly to the bug fix and not include anything unrelated.

## Releasing a New Version

Release branches are used to isolate a stable version of CDI.  Git tags are used within these release branches to track incrementing of Minor and Patch versions.  When a Major version is incremented, a new stable branch should be created corresponding to the release.

- Release branches should adhere to the `release-v#.#.#` pattern.

- Tags should adhere to the `v#.#.#(-alpha.#)` pattern.

When creating a new release branch, follow the below process.  This assumes that `origin` references a fork of `kubevirt/containerized-data-importer` and you have added the main repository as the remote alias `<upstream>`.  If you have cloned `kubevirt/containerized-data-importer` directly, omit the `<upstream>` alias.

1. Make sure you have the latest upstream code

    `$ git fetch <upstream>`

1. Create and checkout the release branch locally

    `$ git checkout -b release-v#.#`

    e.g. `$ git checkout -b release-v1.1`

1. Create an annotated tag corresponding to the version

    `$ git tag -a -m "v#.#.#" v#.#.#`

    e.g. `$ git tag -a -m "v1.1.0" v1.1.0`

1. Push the new branch and tag to the main cdi repo at the same time.  (If you have cloned the main repo directly, use `origin` for <`upstream`>)

    `$ git push <upstream> release-v#.# v#.#.#`

    e.g. `$git push upstream release-v1.1 v1.1.0`

1. Generate release description. Set `PREREF` and `RELREF` shell variables to previous and current release tag, respectively.

    `$ export RELREF=v#.#.#`
    `$ export PREREF=v#.#.#`
    `$ make release-description`

CI will be triggered when a tag matching `v#.#.#` is pushed *AND* the commit changed. So you cannot simply make a new tag and push it, this will not trigger the CI. The automation will handle release artifact testing, building, and publishing.

Following the release, `make release-description` should be executed to generate a github release description template.  The `Notable Changes` section should be filled in manually, briefly listing major changes that the new release includes.  Copy/Paste this template into the corresponding github release.

## Promoting the release
The CI will create the release as a 'pre-release' and as such it will not show up as the latest release in Github. In order to promote it to a regular release go to [CDI Github](https://github.com/kubevirt/containerized-data-importer) and click on releases on the right hand side. This will list all the releases including the new pre-release. Click edit on the pre-release (if you have permission to do so). This will open up the release editor. You can put the release description in the test area field, and uncheck the 'This is a pre-release' checkbox. Click Update release to promote to a regular release.

## Images

Ensure that the new images are available in quay.io/repository/kubevirt/container-name?tab=tags and that the version you specified in the tag is available

* [CDI controller](https://quay.io/repository/kubevirt/cdi-controller?tab=tags)
* [CDI importer](https://quay.io/repository/kubevirt/cdi-importer?tab=tags)
* [CDI cloner](https://quay.io/repository/kubevirt/cdi-cloner?tab=tags)
* [CDI upload proxy](https://quay.io/repository/kubevirt/cdi-uploadproxy?tab=tags)
* [CDI api server](https://quay.io/repository/kubevirt/cdi-apiserver?tab=tags)
* [CDI upload server](https://quay.io/repository/kubevirt/cdi-uploadserver?tab=tags)
* [CDI operator](https://quay.io/repository/kubevirt/cdi-operator?tab=tags)
## PROW CI

Track the CI job for the pushed tag.  Navigate to the [CDI PROW postsubmit dashboard](https://prow.apps.ovirt.org/?repo=kubevirt%2Fcontainerized-data-importer&type=postsubmit) and you can select the releases from there.
