# Documentation System Architecture

Our documentation system is designed to serve both public (external) and internal (engineer-only) content through a single unified API, with access control enforced via user roles.

## 1. Unified Endpoint

All documentation is accessible through the following authenticated routes:

- **Manifests**: `GET /api/v1/gamelift/docs`
- **Content**: `GET /api/v1/gamelift/docs/:slug`

## 2. Role-Based Access Control (RBAC)

The `DocsService` evaluates the user's role (extracted from the `X-User-Role` header or JWT) to determine which content to serve.

### Authorization Logic
| Role | Access Level | Scopes Visible |
| :--- | :--- | :--- |
| `user` | Regular | `public` |
| `admin` | Elevated | `public`, `internal` |
| `system` | Service | `public`, `internal` |

### Content Discovery
When a user requests a `:slug`:
1. The system first checks the `public` folder.
2. If not found and the user is an `admin`/`system`, it checks the `internal` folder.
3. If still not found or if the user is unauthorized for an internal document, a `404` is returned.

## 3. Adding New Documentation

Documentation is organized in two directories under `docs/`:

- `docs/public/`: General guides for integration and API usage.
- `docs/internal/`: Architecture diagrams, engineering conventions, and setup guides.

### To add a new document:
1. Create a `.md` file in the appropriate directory.
2. Add the document to the corresponding `manifest.json`.
3. The system will automatically serve it to authorized users.

## 4. Manifest Structure

The unified manifest endpoint returns an object containing one or more manifest scopes:

```json
{
  "data": {
    "public": { ... },
    "internal": { ... }
  }
}
```
Frontends should check for the presence of the `internal` key to toggle management UI elements.
