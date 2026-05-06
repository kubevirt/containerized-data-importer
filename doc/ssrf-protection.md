# Network Security for HTTP-Based DataVolume Sources

## Overview

CDI includes network access controls for HTTP-based DataVolume sources to prevent unintended access to internal network endpoints.

**Applies to:**
- HTTP/HTTPS imports (`spec.source.http`)
- S3 imports (`spec.source.s3`)
- ImageIO imports (oVirt/RHV) (`spec.source.imageio`)

By default, DataVolume imports are restricted from accessing private network addresses. This helps ensure that import operations only connect to intended external resources.

## Default Network Restrictions

### Restricted IP Ranges

The following CIDR ranges are restricted by default for HTTP-based DataVolume sources:

| Range | Purpose |
|-------|---------|
| `169.254.0.0/16` | Link-local addresses (AWS/Azure/GCP metadata endpoints) |
| `10.0.0.0/8` | RFC 1918 private networks |
| `172.16.0.0/12` | RFC 1918 private networks |
| `192.168.0.0/16` | RFC 1918 private networks |
| `100.64.0.0/10` | Carrier-grade NAT (CGNAT) |
| `127.0.0.0/8` | Loopback addresses |
| `0.0.0.0/8` | Current network |
| `224.0.0.0/4` | Multicast |
| `240.0.0.0/4` | Reserved |
| `fd00::/8` | IPv6 unique local addresses |
| `fe80::/10` | IPv6 link-local |
| `::1/128` | IPv6 loopback |

### Implementation Details

Network access controls are implemented with multiple validation layers:

**For HTTP/HTTPS/S3 sources:**
1. Early URL validation in `NewHTTPDataSource()`/`NewS3DataSource()` - validates the endpoint host before any network calls
2. DialContext hook in HTTP client - validates every TCP connection attempt (defense in depth)

**For ImageIO sources:**
1. Early endpoint validation in `NewImageioDataSource()` - validates the oVirt API endpoint before connection
2. Transfer URL validation in `createImageioReader()` - validates the ImageIO transfer URL returned by oVirt API (prevents compromised oVirt server or DNS rebinding attacks)
3. DialContext hook in HTTP client - validates every connection (defense in depth)

This layered approach ensures that even if one validation layer is bypassed, others remain in place.

## Configuring Allowlist for In-Cluster Services

For legitimate use cases where DataVolumes need to import from internal endpoints (e.g., in-cluster Minio, Ceph RGW, or private registries), cluster administrators can configure an allowlist in the CDIConfig resource.

### Allowlist Configuration

The allowlist is configured in `CDIConfig.spec.allowedSourceURLs` and supports three formats:

1. **CIDR notation** - Allow an entire IP range
2. **Exact IP address** - Allow a specific IP
3. **Hostname** - Allow a hostname and its subdomains (case-insensitive)

**Important:** Only cluster administrators can configure the allowlist. The CDIConfig resource is cluster-scoped, so namespace users cannot bypass SSRF protection.

### Examples

#### Allow Kubernetes Service CIDR

```bash
kubectl patch cdiconfig config --type=merge -p '{
  "spec": {
    "allowedSourceURLs": ["10.96.0.0/12"]
  }
}'
```

#### Allow Multiple In-Cluster Services

```bash
kubectl patch cdiconfig config --type=merge -p '{
  "spec": {
    "allowedSourceURLs": [
      "10.96.0.0/12",
      "minio.storage.svc",
      "ceph-rgw.rook-ceph.svc",
      "192.168.1.100"
    ]
  }
}'
```

#### Check Current Configuration

```bash
kubectl get cdiconfig config -o jsonpath='{.spec.allowedSourceURLs}'
```

### Allowlist Entry Types

#### CIDR Notation

```yaml
allowedSourceURLs:
  - "10.96.0.0/12"      # Kubernetes service CIDR
  - "172.30.0.0/16"     # OpenShift service CIDR
```

Allows any IP within the specified range.

#### Exact IP Address

```yaml
allowedSourceURLs:
  - "192.168.1.100"     # Specific internal server
  - "10.0.50.25"        # Specific endpoint
```

Allows only the exact IP address specified.

#### Hostname Matching

```yaml
allowedSourceURLs:
  - "minio.default.svc"           # In-cluster Minio
  - "registry.example.com"        # Private registry
```

Hostname matching includes:
- Exact match: `minio.default.svc` matches `minio.default.svc`
- Subdomain match: `minio.default.svc` matches `api.minio.default.svc`
- Case-insensitive: `MINIO.DEFAULT.SVC` matches `minio.default.svc`

**Note:** Hostnames are resolved to IPs, and those IPs are validated against the blocklist. An allowlisted hostname that resolves to a public IP is automatically permitted (public IPs are not blocked by default).

## DataVolume Examples

### Restricted Import

Imports from private IP addresses will fail unless explicitly allowed:

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: private-source
spec:
  source:
    http:
      url: http://10.0.0.1/disk.img
  pvc:
    accessModes: [ReadWriteOnce]
    resources:
      requests:
        storage: 1Gi
```

Error message in events:
```
host "10.0.0.1" resolves to blocked IP 10.0.0.1 (SSRF protection)
```

### Public HTTP Import (Works by Default)

This DataVolume works without any configuration because the URL resolves to a public IP:

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: public-image
spec:
  source:
    http:
      url: https://cloud.centos.org/centos/7/images/CentOS-7-x86_64-GenericCloud.qcow2
  pvc:
    accessModes: [ReadWriteOnce]
    resources:
      requests:
        storage: 10Gi
```

### In-Cluster Service Import (Requires Allowlist)

After configuring the allowlist with `minio.default.svc`, this DataVolume will work:

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: minio-import
spec:
  source:
    http:
      url: http://minio.default.svc:9000/bucket/disk.qcow2
  pvc:
    accessModes: [ReadWriteOnce]
    resources:
      requests:
        storage: 5Gi
```

### Private IP Import with CIDR Allowlist

After configuring the allowlist with `10.96.0.0/12`, this DataVolume will work:

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: internal-import
spec:
  source:
    http:
      url: http://10.96.1.100:8080/images/disk.img
  pvc:
    accessModes: [ReadWriteOnce]
    resources:
      requests:
        storage: 20Gi
```

### S3 Import Examples

S3 imports have the same network restrictions as HTTP imports.

**Allowed S3 endpoint with allowlist:**
```yaml
# First configure allowlist
kubectl patch cdiconfig config --type=merge -p '{
  "spec": {
    "allowedSourceURLs": ["10.96.0.0/12"]
  }
}'

# Then create DataVolume
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: allowed-s3
spec:
  source:
    s3:
      url: http://10.96.1.5:9000/bucket/disk.img
      secretRef: s3-credentials
  pvc:
    accessModes: [ReadWriteOnce]
    resources:
      requests:
        storage: 10Gi
```

### ImageIO (oVirt/RHV) Import Examples

ImageIO imports validate both the oVirt API endpoint and the ImageIO transfer URL.

**Allowed oVirt endpoint with allowlist:**
```yaml
# First configure allowlist
kubectl patch cdiconfig config --type=merge -p '{
  "spec": {
    "allowedSourceURLs": ["192.168.1.0/24"]
  }
}'

# Then create DataVolume
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: allowed-imageio
spec:
  source:
    imageio:
      url: http://192.168.1.10/ovirt-engine/api
      secretRef: ovirt-credentials
      diskId: "disk-123"
  pvc:
    accessModes: [ReadWriteOnce]
    resources:
      requests:
        storage: 20Gi
```

### Important: ImageIO Dual Validation

ImageIO imports validate **two separate URLs**:

1. **oVirt Engine API URL** - The endpoint in `spec.source.imageio.url` (e.g., `http://ovirt-engine.example.com/ovirt-engine/api`)
2. **ImageIO Transfer URL** - Returned by the oVirt API during image transfer setup (e.g., `http://imageio-proxy.example.com:54323`)

**Both URLs must pass SSRF validation.** If your oVirt deployment uses different hostnames or IPs for the engine API and ImageIO transfer service, **both must be in the allowlist**.

#### Common ImageIO Deployment Patterns

**Pattern 1: Same hostname for engine and transfer**
```yaml
allowedSourceURLs:
  - "ovirt-engine.example.com"  # Covers both engine API and transfer URL
```

**Pattern 2: Separate hostnames**
```yaml
allowedSourceURLs:
  - "ovirt-engine.example.com"   # Engine API
  - "imageio-proxy.example.com"  # ImageIO transfer service
```

**Pattern 3: IP-based with CIDR**
```yaml
allowedSourceURLs:
  - "192.168.1.0/24"  # Covers both engine (192.168.1.10) and transfer URLs in same subnet
```

**Pattern 4: Mixed hostname and IP**
```yaml
allowedSourceURLs:
  - "ovirt-engine.example.com"  # Engine API hostname
  - "192.168.1.0/24"            # Transfer URLs may use IPs
```

#### Troubleshooting ImageIO Validation

If an ImageIO import fails with "transfer URL validation failed", check the importer pod logs:

```bash
kubectl logs -l cdi.kubevirt.io/dataVolume=<datavolume-name>
```

Look for the actual transfer URL that oVirt returned:
```
Error: transfer URL validation failed: host "192.168.1.50:54323" resolves to blocked IP 192.168.1.50
```

Then add the transfer URL's hostname or IP range to the allowlist:
```bash
kubectl patch cdiconfig config --type=merge -p '{
  "spec": {
    "allowedSourceURLs": [
      "ovirt-engine.example.com",
      "192.168.1.0/24"
    ]
  }
}'
```

## Validation Behavior

### Validation Order

For each HTTP/HTTPS import:

1. **Early URL validation** - The destination hostname/IP is validated before any network calls
2. **DNS resolution** - The hostname is resolved to IP addresses
3. **Allowlist check** - If any resolved IP matches an allowlist entry, the import proceeds
4. **Blocklist check** - If any resolved IP matches a blocked range (and wasn't allowlisted), the import fails
5. **Connection validation** - The DialContext hook validates each connection attempt (defense in depth)

### Multiple IP Addresses

If a hostname resolves to multiple IP addresses:
- **At least one IP must be allowed** - If any resolved IP matches the allowlist or is a public IP, the import proceeds
- **All IPs are checked** - If all resolved IPs are blocked (and none are allowlisted), the import fails

Example: `internal.example.com` resolves to `[10.0.0.1, 8.8.8.8]`
- Without allowlist: **Allowed** (8.8.8.8 is public)
- With allowlist `["10.0.0.0/8"]`: **Allowed** (10.0.0.1 matches allowlist)

## HTTP Proxy Support

Network access controls are **compatible with HTTP proxy support**. CDI continues to respect `HTTP_PROXY` and `HTTPS_PROXY` environment variables for importer pods.

The validation works by checking the **destination URL** before making network calls, so restricted destinations are rejected early even when using a proxy.

**Important:** If you allowlist a hostname or IP range, ensure that your HTTP proxy implements appropriate egress controls.

## Configuration Security

### Allowlist Permissions

- **Cluster-admin only**: Only users with write access to the cluster-scoped CDIConfig resource can modify the allowlist
- **Namespace users cannot override**: Regular users cannot add allowlist entries
- **Global allowlist**: The allowlist applies to all namespaces in the cluster (there is no per-namespace allowlist)

### Recommendations

1. **Minimize allowlist entries**: Only add entries for services that genuinely need to be accessible via DataVolume HTTP imports

2. **Use narrow CIDR ranges**: Instead of allowing entire private ranges, use specific subnets:
   - Good: `10.96.0.0/12` (Kubernetes service CIDR)
   - Avoid: `10.0.0.0/8` (entire private range)

3. **Prefer hostnames over IPs**: Hostname-based allowlist entries are more maintainable as infrastructure changes

4. **Monitor for failures**: Watch for DataVolume failures with "SSRF protection" errors, which may indicate legitimate use cases that need allowlisting

5. **Audit allowlist changes**: Track changes to CDIConfig.spec.allowedSourceURLs in your audit logs

## Troubleshooting

### DataVolume Stuck in "ImportScheduled" with Network Error

**Symptom:** DataVolume events show:
```
SSRF protection: host "10.0.0.1" resolves to blocked IP 10.0.0.1
```

**Solution:** The destination IP is blocked. If this is a legitimate internal endpoint:

```bash
# Add the IP or CIDR to the allowlist
kubectl patch cdiconfig config --type=merge -p '{
  "spec": {
    "allowedSourceURLs": ["10.0.0.0/24"]
  }
}'

# Delete and recreate the DataVolume
kubectl delete dv <datavolume-name>
kubectl apply -f <datavolume.yaml>
```

### Allowlist Not Taking Effect

**Symptom:** Added allowlist entry but DataVolume still fails with SSRF error

**Check:**

1. Verify the allowlist was applied:
   ```bash
   kubectl get cdiconfig config -o yaml | grep -A 10 allowedSourceURLs
   ```

2. Check importer pod logs for allowlist confirmation:
   ```bash
   kubectl logs -l cdi.kubevirt.io/dataVolume=<datavolume-name> | grep allowlist
   ```
   
   Expected output:
   ```
   SSRF protection allowlist: [10.96.0.0/12 minio.default.svc]
   ```

3. Ensure the DataVolume was created **after** the allowlist was configured (existing pods don't pick up config changes)

### Hostname Resolves to Blocked IP

**Symptom:** DataVolume with hostname URL fails even though hostname is allowlisted

**Example:**
```yaml
allowedSourceURLs: ["internal.example.com"]
```

DataVolume with `http://internal.example.com/disk.img` fails with:
```
SSRF protection: host "internal.example.com" resolves to blocked IP 10.0.0.1
```

**Root Cause:** The hostname `internal.example.com` is allowlisted, but it resolved to a blocked IP (`10.0.0.1`). Hostname allowlisting permits the hostname itself, but the resolved IP must also be allowed.

**Solution:** Add the CIDR range or exact IP to the allowlist:
```bash
kubectl patch cdiconfig config --type=merge -p '{
  "spec": {
    "allowedSourceURLs": ["internal.example.com", "10.0.0.0/24"]
  }
}'
```

## Migration from Older Versions

### Upgrading CDI

**Impact:** Existing DataVolumes that import from **private IP addresses** will fail after upgrading unless configured in the allowlist.

**Migration Steps:**

1. **Before upgrade**: Audit existing DataVolumes for private IP usage:
   ```bash
   kubectl get dv -A -o json | \
     jq -r '.items[] | select(.spec.source.http.url | test("^https?://(10\\.|172\\.(1[6-9]|2[0-9]|3[01])\\.|192\\.168\\.|169\\.254\\.|127\\.)")) | "\(.metadata.namespace)/\(.metadata.name): \(.spec.source.http.url)"'
   ```

2. **After upgrade**: Configure allowlist for legitimate internal endpoints

3. **Re-create affected DataVolumes**: DataVolumes created before the allowlist was configured need to be deleted and recreated

### Example Migration

**Before upgrade**: DataVolume importing from in-cluster Minio at `10.96.1.5`:
```yaml
spec:
  source:
    http:
      url: http://10.96.1.5:9000/bucket/disk.qcow2
```

**After upgrade** (without allowlist): ❌ Fails with SSRF error

**After adding allowlist**:
```bash
kubectl patch cdiconfig config --type=merge -p '{
  "spec": {
    "allowedSourceURLs": ["10.96.0.0/12"]
  }
}'
```

**Re-create DataVolume**: ✅ Now works

## Related Documentation

- [CDI Configuration](cdi-config.md)
- [DataVolumes](datavolumes.md)
- [Import from HTTP/S3](datavolumes.md#http-source)
