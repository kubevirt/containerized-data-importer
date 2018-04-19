# Versioning

This project follows the symantic versioning guidelines laid out in https://semver.org/ .

The project's version is tracked in the [version](/version) file.
### Developers

It is the responsibility of the project's maintainers to increment the version manually.  This *must* be done each time a PR is merged into the _master_ branch.  Incrementation must be done in accordance with the below definitions.

### Definitions

- Major versions represent core API changes or features that are _not backwards compatible_.  CDI does not expose an API itself but does rely on the Kubernetes API.  As such, Changes in CDI that significantly alter the way the Kubernetes API is used (change in resource types, implementation of a CRD, etc) will warrant an increment of the major version.

- Minor versions represent the addition of _backwards compatible_ features to CDI. Such features must not alter the way a user will interact with CDI.  Examples may include changes to manifests (that do not affect their structure), addtional data format handling, controller behavior, etc.

- Patch versions represent changes to the code that do not add features or affect the way a user may interact with CDI.  Such changes may be bug fixes, README additions or changes, refactors, etc.
