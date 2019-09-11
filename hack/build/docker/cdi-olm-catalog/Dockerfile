FROM quay.io/openshift/origin-operator-registry:4.2.0

RUN mkdir -p /tmp/olm-catalog
COPY olm-catalog /tmp/olm-catalog
RUN chown 1001 /tmp/olm-catalog

# Initialize the database
RUN initializer --manifests /tmp/olm-catalog --output bundles.db

# There are multiple binaries in the origin-operator-registry
# We want the registry-server
ENTRYPOINT ["registry-server"]
CMD ["--database", "bundles.db"]

