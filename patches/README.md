#### `fix-replace-op-for-evanphx-json-patch-v5-lib.patch`
Fix JSON patch replace operation behaviour: set value even if path is not exist.

Why we need it?

Previous CDI version uses evanphx-json-patch version 5.6.0+incompatible. That version ignores non-existent path and it actually a v4 from main branch.

CDI 1.60.3 uses evanphx-json-patch /v5 version, which has options to change the behaviour for add and remove operations, but there is no option to change behaviour for the replace operation.