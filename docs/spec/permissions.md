# Permission Matrix (v0.1)

Normative authorization contract for fixed roles from ADR-032.

Roles:

- `admin`
- `user`
- `auditor`

Legend:

- `YES`: permitted
- `NO`: denied
- `SELF`: only own identity/resource scope
- `SCOPE`: constrained to assigned project/resource boundary

## Helling API (`/api/v1/*`)

| Endpoint Group                         | admin                 | user                         | auditor         |
| -------------------------------------- | --------------------- | ---------------------------- | --------------- |
| auth setup/login/refresh/logout        | YES                   | YES                          | YES             |
| auth totp management                   | SELF                  | SELF                         | SELF            |
| auth token list/create/revoke          | SELF + admin override | SELF                         | SELF            |
| users list/get                         | YES                   | SELF                         | YES (read-only) |
| users create/update/delete             | YES                   | NO                           | NO              |
| schedules list/get                     | YES                   | SCOPE                        | YES (read-only) |
| schedules create/update/delete/run     | YES                   | SCOPE                        | NO              |
| webhooks list/get                      | YES                   | NO                           | YES (read-only) |
| webhooks create/update/delete/test     | YES                   | NO                           | NO              |
| kubernetes list/get                    | YES                   | SCOPE                        | YES (read-only) |
| kubernetes create/delete/scale/upgrade | YES                   | SCOPE                        | NO              |
| kubernetes kubeconfig                  | YES                   | SCOPE                        | YES (read-only) |
| system info/hardware/diagnostics       | YES                   | NO                           | YES (read-only) |
| system config read                     | YES                   | NO                           | YES (read-only) |
| system config write/upgrade            | YES                   | NO                           | NO              |
| firewall host list                     | YES                   | NO                           | YES (read-only) |
| firewall host create/delete            | YES                   | NO                           | NO              |
| audit query/export                     | YES                   | SELF (query own events only) | YES             |
| events SSE                             | YES                   | YES (filtered)               | YES (filtered)  |
| health                                 | YES                   | YES                          | YES             |

## Incus Proxy (`/api/incus/*`)

Helling enforces role gate, then forwards with caller-specific Incus client certificate identity (ADR-024 + ADR-036).

| Method Class                                | admin | user  | auditor                            |
| ------------------------------------------- | ----- | ----- | ---------------------------------- |
| Read (`GET`)                                | YES   | SCOPE | SCOPE (read-only cert constraints) |
| Mutation (`POST`, `PUT`, `PATCH`, `DELETE`) | YES   | SCOPE | NO                                 |

`SCOPE` is ultimately enforced by Incus trust restrictions on the certificate presented by hellingd.

## Podman Proxy (`/api/podman/*`)

Role gate is enforced in Helling middleware prior to proxying.

| Method Class                                | admin | user | auditor |
| ------------------------------------------- | ----- | ---- | ------- |
| Read (`GET`)                                | YES   | YES  | YES     |
| Mutation (`POST`, `PUT`, `PATCH`, `DELETE`) | YES   | YES  | NO      |

## Notes

- Endpoint-specific exceptions must be documented in api/openapi.yaml operation description.
- Authorization failures return `AUTH_INVALID_TOKEN`, `AUTH_INVALID_CREDENTIALS`, or domain-specific forbidden errors from docs/spec/errors.md.
