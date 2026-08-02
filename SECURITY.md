# Security Policy

## Supported versions

Adaptive Search has not yet cut a tagged release. Security fixes land on `main`,
and `main` is the only supported version.

## Reporting a vulnerability

**Please do not open a public issue for a security vulnerability.**

Report it privately through GitHub's coordinated disclosure flow:

> **[Open a private security advisory](https://github.com/iampopye/IMI_Hackathon/security/advisories/new)**

If that is not available to you, contact the maintainer through
[@iampopye](https://github.com/iampopye) and ask for a private channel before
sharing any details.

### What to include

- The affected component (`search-system` API, the write pipeline, the
  `search-ui` dashboard, the Kubernetes manifests, or the Docker Compose stack)
- The version or commit SHA you tested
- Reproduction steps, ideally a minimal request sequence
- The impact you believe it has

### What to expect

This is a single-maintainer project, so please calibrate accordingly:

| Stage | Target |
| --- | --- |
| Acknowledgement of your report | within 5 working days |
| Initial assessment and severity | within 10 working days |
| Fix or documented mitigation | depends on severity and complexity |

You will be credited in the advisory unless you ask not to be.

## Scope

In scope:

- Authentication and authorisation gaps in the HTTP API
- SQL injection, command injection, or path traversal
- Secrets exposed by the application at runtime or in logs
- Denial of service reachable through a normal API request
- Dependency vulnerabilities that are actually reachable from this codebase

Out of scope — these are known and intentional properties of the current build,
not vulnerabilities. They are documented in the README under
"Security posture and production readiness":

- **The API ships with no authentication.** Every endpoint is unauthenticated by
  design in this build. Do not expose it to an untrusted network.
- **CORS is `Access-Control-Allow-Origin: *`.** This is set in
  `search-system/internal/api/router.go` so the local dashboard can call the API.
- **The default credentials in `docker-compose.yml`, `.env.example` and
  `config.yaml.example` are development placeholders** (`searchuser`/`searchpass`,
  Grafana `admin`/`admin`). They are meant to be replaced. Reporting them as
  leaked credentials is not a finding.
- **`PostgresConfig.DSN()` sets `sslmode=disable`**, intended for the local
  Compose stack.
- Findings from an automated scanner with no demonstrated impact on this code.

If you think one of the above is exploitable in a way the README does not
describe, that is worth reporting — say which assumption you are breaking.

## Hardening notes for operators

If you run this yourself, at minimum:

- Put an authenticating reverse proxy in front of the API, or keep it on a
  private network
- Replace every default password, and inject secrets via the environment
  variables the config layer already supports (`POSTGRES_PASSWORD`,
  `REDIS_PASSWORD`, `ES_PASSWORD`)
- Enable TLS by setting `TLS_CERT_FILE` and `TLS_KEY_FILE`
- Restrict the CORS origin to the host that actually serves the dashboard
- Do not expose Prometheus (`:9090`), Grafana (`:3000`), Elasticsearch (`:9200`),
  Kibana (`:5601`) or Kafka (`:9092`) publicly — the Compose file publishes all
  of them to the host for local development convenience
