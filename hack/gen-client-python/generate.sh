#!/usr/bin/env bash

source $(dirname "$0")/../build/common.sh

set -o errexit
set -o nounset
set -o pipefail

SWAGGER_CODEGEN_CLI_SRC=http://central.maven.org/maven2/io/swagger/swagger-codegen-cli/2.2.3/swagger-codegen-cli-2.2.3.jar
SWAGGER_CODEGEN_CLI="/tmp/swagger-codegen-cli.jar"
CDI_SPEC="${CDI_DIR}/api/openapi-spec/swagger.json"
CODEGEN_CONFIG_SRC="${CDI_DIR}/hack/gen-client-python/swagger-codegen-config.json.in"
CODEGEN_CONFIG="${PYTHON_CLIENT_OUT_DIR}/swagger-codegen-config.json"

# Define version of client
if [ -n "${TRAVIS_TAG:-}" ]; then
    CLIENT_PYTHON_VERSION="$TRAVIS_TAG"
else
    CLIENT_PYTHON_VERSION="$(git describe || echo 'none')"
fi

rm -rf "${PYTHON_CLIENT_OUT_DIR}"

mkdir -p "${PYTHON_CLIENT_OUT_DIR}"

# Download swagger code generator
curl "$SWAGGER_CODEGEN_CLI_SRC" -o "$SWAGGER_CODEGEN_CLI"

# Generate config file for swagger code generator
sed -e "s/[\$]VERSION/${CLIENT_PYTHON_VERSION}/" \
    "${CODEGEN_CONFIG_SRC}" >"${CODEGEN_CONFIG}"

# Generate python client
java -jar "$SWAGGER_CODEGEN_CLI" generate \
    -i "$CDI_SPEC" \
    -l python \
    -o "${PYTHON_CLIENT_OUT_DIR}" \
    --git-user-id kubevirt \
    --git-repo-id cdi-client-python \
    --release-note "Auto-generated client ${CLIENT_PYTHON_VERSION}" \
    -c "${CODEGEN_CONFIG}" &>"${PYTHON_CLIENT_OUT_DIR}"/cdi-pysdk-codegen.log
