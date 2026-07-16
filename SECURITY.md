# Security Policy

## Supported version

Security fixes are applied to the latest revision on the default branch while miniACS remains in beta.

## Report a vulnerability

Please use GitHub's **Private vulnerability reporting** feature for this repository. Include the affected revision, deployment topology, reproduction steps, and expected impact. Do not include real subscriber credentials, firmware images, or device exports.

If private reporting is unavailable, contact the repository owner privately before publishing technical details. Please allow reasonable time for triage and remediation.

## Deployment baseline

- Generate a unique `JWT_SECRET` of at least 32 random characters.
- Generate and back up a unique `PARAMETER_ENCRYPTION_KEY`; do not rotate it without re-encrypting existing data.
- Rotate or remove `INITIAL_ADMIN_PASSWORD` after the first login.
- Terminate browser traffic with HTTPS and restrict API CORS origins.
- Configure `TRUSTED_PROXY_CIDRS` and `CWMP_TRUSTED_PROXY_CIDRS` with proxy addresses only.
- Set `CONNECTION_REQUEST_ALLOWED_CIDRS` to the managed CPE destination networks.
- Restrict the CWMP listener to known CPE networks; CWMP Basic Auth must only run behind TLS.
- Keep PostgreSQL and the firmware storage directory off the public network.
- Set `firmware_base_url` to an HTTPS endpoint reachable by managed CPEs.
- Back up the database before bulk provisioning, reset, or firmware operations.

Signed firmware links are time-limited bearer credentials. They should not be logged by reverse proxies and must only be shared with the intended CPE. Firmware authenticity remains vendor-specific; operators must validate checksums/signatures and compatibility before scheduling an upgrade.
