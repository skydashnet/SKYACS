# Security Policy

## Supported version

Security fixes are applied to the latest revision on the default branch while miniACS remains in beta.

## Report a vulnerability

Please use GitHub's **Private vulnerability reporting** feature for this repository. Include the affected revision, deployment topology, reproduction steps, and expected impact. Do not include real subscriber credentials, firmware images, or device exports.

If private reporting is unavailable, contact the repository owner privately before publishing technical details. Please allow reasonable time for triage and remediation.

## Deployment baseline

- Generate a unique `JWT_SECRET` of at least 32 random characters.
- Rotate or remove `INITIAL_ADMIN_PASSWORD` after the first login.
- Terminate browser traffic with HTTPS and restrict API CORS origins.
- Restrict the CWMP listener to known CPE networks; enable CWMP authentication where supported.
- Keep PostgreSQL and the firmware storage directory off the public network.
- Set `firmware_base_url` to an HTTPS endpoint reachable by managed CPEs.
- Back up the database before bulk provisioning, reset, or firmware operations.

Signed firmware links are bearer credentials. They should not be logged by reverse proxies and must only be shared with the intended CPE.
