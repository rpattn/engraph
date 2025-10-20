# Go Server

A starting **Go Server** built with:

- Multi-tenant architecture (users, organizations, roles).
- Authentication via **username/password**, **TOTP MFA**, and **OAuth (Google, Microsoft)**.
- Role-based access control (Owner, Admin, Member, Viewer).
- PostgreSQL-backed persistence, generated queries with **sqlc**.
- Database migrations with **golang-migrate**.
- Configurable via `config.yaml` or environment variables.
- HTTP server built on **chi**, secure session cookies, and static asset serving.
- Modular design for scaling into a full CMMS with transaction logging.

---

## ⚡ Quickstart

1. **Start dependencies**
   - Postgres: `docker run --name engraph-postgres -e POSTGRES_PASSWORD=postgres -p 5432:5432 -d postgres:14`
   - TerminusDB: `docker run --name engraph-terminus -e TERMINUSDB_ADMIN_PASS=supersecret -p 6363:6363 -d terminusdb/terminusdb-server:latest`
2. **Copy the example configuration** and adjust database, TerminusDB, and session settings:
   ```bash
   cp example.config.yaml config.yaml
   ```
   Make sure the Terminus credentials match how you started the container (the container password flag is `TERMINUSDB_ADMIN_PASS`). If you used the command above, set:

   ```yaml
   terminus:
     user: admin
     password: supersecret
   ```

   Alternatively you can supply `TERMINUS_TOKEN` if you are using TerminusDB Cloud; the service accepts either a bearer token or basic auth credentials for each request.
3. **Run database migrations** to set up auth and metadata tables:
   ```bash
   make migrate-up
   ```
4. **Seed demo property definitions in TerminusDB** so the playground mutation works. Grab the organisation UUID from Postgres (the seed data creates an `acme` org) and replace `<org-uuid>` in [`docs/terminus/sample_property_definitions.json`](docs/terminus/sample_property_definitions.json) with that value.

   ```bash
   psql postgres://postgres:admin@localhost:5432/db -At -c "SELECT id FROM organisations WHERE slug = 'acme' LIMIT 1;"

   curl -u admin:supersecret \
     -H "Content-Type: application/json" \
     -H "Prefer: return=representation" \
     -X POST "http://localhost:6363/api/document/engraph/entities?branch=main" \
     --data-binary @docs/terminus/sample_property_definitions.json
   ```

   Repeat the `curl` command for each org (update the `org_id` value) if you want to preload definitions across tenants.
5. **Start the API server**:
   ```bash
   go run ./cmd/server
   ```
6. **Open the GraphQL Playground** at [http://localhost:8080/graphql/playground](http://localhost:8080/graphql/playground). The playground is scoped by your login session, so sign in first using the existing auth endpoints or the sample forms under `/static/test.html`.

> **Troubleshooting:** A `401 Authorization Required` error from TerminusDB usually means the password in `config.yaml` does not match the value supplied via `TERMINUSDB_ADMIN_PASS` when starting the container.

### Sample GraphQL operations

Use these documents directly in the playground to exercise the Terminus-backed entity API once you are authenticated.

**Query entities**

```graphql
query ListParts {
  entities(filter: { type: "Part" }, limit: 10) {
    id
    name
    description
    customProperties {
      name
      value
      refValue {
        id
        name
      }
    }
    relationships {
      name
      target {
        id
        name
      }
    }
  }
}
```

**Create an entity**

```graphql
mutation CreateEntity {
  createEntity(
    input: {
      type: "Part"
      name: "Widget"
      description: "Demo component"
      customProperties: [
        { name: "color", value: "blue" }
        { name: "supplier", refValueId: "<entity-id>" }
      ]
    }
  ) {
    id
    name
    customProperties {
      name
      value
    }
  }
}
```

Replace `<entity-id>` with an entity identifier returned from a previous query when testing relationship-backed properties.

> Want to try different fields? Add more property definition documents in TerminusDB. Copy the sample JSON, tweak the `property_name`, `property_type`, or `ref_target_type`, and POST it to the Terminus `document` endpoint with the matching `org_id`. Once the definition exists you can send the property in `customProperties` and it will be validated and stored in TerminusDB.

---

## 🚀 Project Goals

- Provide a robust authentication/authorization foundation.
- Support organizations, roles, and multi-tenant security.

---

## 🏗 Project Architecture

<pre>
go_server/
│
├── cmd/
│ └── server/ # Entrypoint for the HTTP server (main.go)
│
├── internal/
│ ├── auth/ # Auth flows: signup, login, logout, MFA (TOTP), OAuth
│ ├── config/ # Config loader (env + YAML)
│ ├── middleware/ # Session and logging middleware
│ ├── models/ # Domain models (User, Org, Role, Credential, etc.)
│ ├── providers/ # OAuth providers (Google, Microsoft)
│ └── repo/ # Repository implementation (wraps sqlc generated code)
│
├── database/
│ ├── migrations/ # SQL migrations (with golang-migrate)
│ ├── queries/ # Handwritten SQL queries for sqlc
│ └── gen/ # sqlc-generated Go code
│
├── static/ # Static assets (test.html)
│
├── scripts/ # PowerShell scripts for install, migrate, sqlc, run
│
├── examle.config.yaml # Default config (example)
└── go.mod # Go module definition
</pre>

---

## 🔑 Authentication Features

- **Local auth**
  - Signup with email, username, password.
  - Passwords hashed with **Argon2id** in PHC string format.
  - TOTP MFA setup/verification (Google Authenticator-compatible).
- **OAuth**
  - Microsoft and Google login support.
  - Automatic user creation via verified email.
  - Organization role mapping from IdP groups.
- **Sessions**
  - Secure session cookies (`HttpOnly`, `Secure`, `SameSite`).

---

## ⚙️ Setup & Development

### Prerequisites
- Go 1.22+
- PostgreSQL 14+
- [sqlc](https://sqlc.dev) (query codegen)
- [golang-migrate](https://github.com/golang-migrate/migrate) (migrations)
- On Windows: use [scoop](https://scoop.sh) for easy installation.

### Install tools
```powershell
scoop install sqlc migrate
```

### Run migrations
`.\scripts\migrate-up.ps1`

### Down migrations
`.\scripts\migrate-down.ps1`

### Generate sql code
`.\scripts\sqlc-generate.ps1`

### Run server
`.\scripts\migrate-up.ps1`

Visit http://localhost:8080/static/test.html
 to use the test UI.


## Configure

Edit config.yaml (or use env vars):

base_url: "http://localhost:8080"

database:
  url: "postgres://postgres:postgres@localhost:5432/cmms?sslmode=disable"

oauth:
  google:
    client_id: "xxx"
    client_secret: "xxx"
  microsoft:
    client_id: "xxx"
    client_secret: "xxx"


## Test

Testing the API

POST /auth/signup → signup new user.

POST /auth/login → login with username/password (+ TOTP if enabled).

POST /auth/logout → clear session cookie.

GET /auth/mfa/totp/setup → provision TOTP secret + QR.

POST /auth/mfa/totp/verify → validate TOTP setup.

A sample test.html is served at /static/test.html with forms for signup/login and logs to browser console.

Users can sign up without providing an org, and they are assigned to the test acme org (see see_demo.(up/down).sql to change this)

Users can also sign up without an org and we try to map from their email domain (e.g. @testorg.com --> testorg slug)

## Why Go?

Performance: Compiles to a single static binary, fast concurrency with goroutines.

Safety: Strong typing, no hidden magic.

Ecosystem: sqlc, chi, pgx, pquerna/otp → strong libraries with minimal runtime overhead.

Deployment: Easy to ship anywhere (Docker, bare metal, cloud).

Scalability: Well-suited for multi-tenant SaaS and high-concurrency APIs.
