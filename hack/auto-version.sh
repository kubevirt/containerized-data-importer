#!/usr/bin/env bash

# auto-version.sh
# This script will be executed by Travis CI upon the push of a new git tag to mark a new version of CDI.
# When a new git tag is pushed to master, Travis will execute this script and pass in the tag value.
# This script will replace the existing version values in known files, then commit the changes and push to master.
# Next it will delete the human created tag in the remote repo, generate a new, identical tag for the
# current commit, and push this tag to master.
# This secondary push is required because the human created tag will reference 1 commit behind the new
# version values.  It is necessary to shift this tag to the commit reflecting the new version.
# This will cause CI to execute again.  auto-version.sh will not execute if the passed in tag matches the
# existing version and will exit with code 0.
#
# Parameters:
#   $1: TRAVIS_TAG (git tag string)

set -eou pipefail

# doIncrement replaces the oldVersion value with the newVersion in the given file
function setNewVersion(){
    local file=$1
    local oldVersion=$2
    local newVersion=$3

    sed -i "s#$oldVersion#$newVersion#" $file
}

# pushNewVersion indexes only the files where versions are known to be specified,
# commits the changes, and pushes to master
# Parameters:
#     $@: Known files containing an updated version value
function pushNewVersion(){
    git add $@
    git commit -m "[ci skip] CI: updated version"
    git tag -f -a -m "[ci skip] CI tagged commit" $NEW_RELEASE_TAG
    git push --tags -f
}

# shiftTag deletes the human defined tag in the remote repo
# and sets it again for the current commit. After the CI has updated
# the version values in the project, a new commit will be pushed to origin.
# This will cause the human defined tag to fall 1 behind the commit where the
# values are changed.  Thus, it is necessary to "shift" the tag by one.
# Note: this initiates a 2nd run of the CI, which is short circuited below
# by comparing tags.
function shiftTag(){
    local versionTag=$1

    git push --delete origin $versionTag # delete the remote stale tag
    git tag -f -a -m "[ci skip] shift existing tag to HEAD" $versionTag
    git push origin $versionTag
}

################
#     MAIN     #
################
# containerized-data-importer/
REPO_ROOT=$(readlink -f $(dirname $0)/../)

# All files where the version is specified
VERSION_FILE="$REPO_ROOT/version"
COMMON_VARS="$REPO_ROOT/pkg/common/common.go"
CONTROLLER_MANIFEST="$REPO_ROOT/manifests/controller/cdi-controller-deployment.yaml"
IMPORTER_MANIFEST="$REPO_ROOT/manifests/importer/importer-pod.yaml"

# Array of target files for iterative ops
TARGET_FILES=($COMMON_VARS $VERSION_FILE $CONTROLLER_MANIFEST $IMPORTER_MANIFEST)

NEW_RELEASE_TAG=${1:-}
OLD_RELEASE_TAG=$(cat "$VERSION_FILE")

if [[ "$OLD_RELEASE_TAG" == "$NEW_RELEASE_TAG" ]]; then
    printf "Version %s matches tag %s: skipping.\n" "$OLD_RELEASE_TAG" "$NEW_RELEASE_TAG"
elif [ -z "$NEW_RELEASE_TAG" ]; then
    printf "No new tag found, shifting most current tag (%s) to now\n" $OLD_RELEASE_TAG
    shiftTag $OLD_RELEASE_TAG
else
    printf "New tag detected (%s)! Updating version values in \n" $NEW_RELEASE_TAG
    for f in ${TARGET_FILES[*]}; do
        printf "%s\n" $f
        setNewVersion $f $OLD_RELEASE_TAG $NEW_RELEASE_TAG
    done
    pushNewVersion ${TARGET_FILES[@]}
fi
