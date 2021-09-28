#!/bin/sh
#
# Check for less inclusive language usage.
# Allowed exceptions (hard to change) excluded with grep -v.
# Generated files excluded.
#

CDI_DIR="$(cd $(dirname $0)/../../ && pwd -P)"

PHRASES='master|slave|whitelist|blacklist'

VIOLATIONS=$(git grep -iI -E $PHRASES -- \
	':!vendor' \
	':!cluster-up' \
	':!cluster-sync' \
	':!*generated*' \
	':!*swagger.json*' \
	':!hack/gen-swagger-doc/deploy.sh' \
	':!hack/ci/language.sh' \
		"${CDI_DIR}" \
		| grep -v \
			-e 'ekalinin/github-markdown-toc' \
			-e 'github.com/kubernetes' \
			-e 'travis-ci/gimme' \
			-e 'node01.*Ready' \
			-e 'actions/checkout' \
			-e 'coverallsapp/github-action')
			# Allowed exceptions

if [ ! -z "${VIOLATIONS}" ]; then
	echo "ERROR: Found new additions of non-inclusive language ${PHRASES}"
	echo "${VIOLATIONS}"
	echo ""
	echo "Please consider different terminology if possible."
	echo "If necessary, an exception can be added to to the hack/ci/language.sh script"
	exit 1
fi
