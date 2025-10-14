![Release Tracker](docs/images/logo.jpeg)

# Release Tracker

A Go application that tracks container image deployments across Kubernetes namespaces, providing a web interface to monitor and visualize release history.

## Features

- **Kubernetes Integration**: Monitors Deployments, StatefulSets, DaemonSets, and ReplicaSets across specified namespaces automatically
- **Data Storage**: SQLite database with automatic deduplication and retention (10 most recent releases per single component)
- **REST API**: Endpoints for triggering collection, retrieving current releases, and accessing release history
- **Web Interface**:
  - Dashboard with hierarchical table view and full-text search
  - Timeline page with release history visualization and cross-environment comparison
  - Environment comparison page for analyzing ALL components across ALL environments
  - Badge viewer to generate shields.io style badges for READMEs
  - Visual difference detection with color-coded indicators (green for matching, yellow for different, gray for missing)
  - Authentication support for API keys and badges
  - Multi-client, multi-environment support
- **Master and Slave Modes**: Centralized monitoring and control for multi-cluster deployments. [Read the full guide](docs/MASTER_MODE_GUIDE.md)

![Release Tracker Dashboard](docs/images/master_screen.png)

## Architecture

### Master Mode (in-cluster)
```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Slave Mode    │    │   Slave Mode    │    │   Master Mode   │
│   Client A      │───▶│   Client B      │───▶│   Aggregator    │
│   - dev         │    │   - staging     │    │   - All Clients │
│   - prod        │    │   - prod        │    │   - All Envs    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```


## Quick Start

### Local Development

#### Prerequisites

- Go 1.21+
- Docker (for containerization)
- Kubernetes cluster with appropriate RBAC permissions

1. **Clone and setup**:
   ```bash
   # Clone the repository first
   cd krelease-tracker
   go mod tidy
   ```

2. **Run locally** (requires kubeconfig):
   ```bash
   export NAMESPACES="default,kube-system"
   export IN_CLUSTER="false"
   export DATABASE_PATH="./releases.db"
   go run cmd/server/main.go
   ```

3. **Access the application**:
   - Dashboard: http://localhost:8080
   - Timeline: http://localhost:8080/timeline.html
   - Badge Viewer: http://localhost:8080/badges.html
   - API: http://localhost:8080/api/

### Kubernetes Deployment with Helm

```bash
# Add Helm repository
helm repo add rogosprojects https://rogosprojects.github.io/helm
# Update Helm repositories
helm repo update
# Install
helm install krelease-tracker rogosprojects/krelease-tracker
```
**Done!** Follow the post-installation instructions to access the application.

[Read the full Helm chart documentation with default values.yaml](https://rogosprojects.github.io/helm/charts/krelease-tracker/)

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `DATABASE_PATH` | `/data/releases.db` | SQLite database file path |
| `NAMESPACES` | `default` | Comma-separated list of namespaces to monitor |
| `COLLECTION_INTERVAL` | `60` | Collection interval in minutes |
| `IN_CLUSTER` | `true` | Whether running inside Kubernetes cluster |
| `KUBECONFIG` | `""` | Path to kubeconfig file (for out-of-cluster) |
| `API_KEYS` | `""` | Comma-separated list of API keys for authentication (optional) |
| `ENV_NAME` | `unknown` | Environment name displayed in badges (e.g., "production", "staging") |
| `BASE_PATH` | `""` | Base path for serving (e.g., "/tracker" for ingress with path prefix) |
| `MODE` | `slave` | Application mode: "master" or "slave" |
| `MASTER_URL` | `""` | Master URL for sync (slave mode only) |
| `MASTER_API_KEY` | `""` | Master API key for sync (slave mode only) |
| `SYNC_INTERVAL` | `5` | Sync interval in minutes (slave mode only) |
| `PROXY_URL` | `""` | HTTP/HTTPS proxy URL for sync requests (slave mode only) |
| `TLS_INSECURE` | `false` | Skip TLS certificate verification for sync requests (slave mode only) |


## API Authentication

Release Tracker supports optional API key authentication for all REST API endpoints with client-based access control. When API keys are configured, all `/api/*` endpoints require authentication.

### API Key Types

**1. Admin API Keys** - Full access to all clients and environments
- Format: `{adminAuth}` (no client prefix)
- Example: `admin-secret-key-12345678901234567890`
- Access: Can view and manage all clients and environments

**2. Standard API Keys** - Restricted access to specific client name
- Format: `{clientName}-{clientAuth}` (exactly one hyphen separator)
- Example: `client1-authkey12345678901234567890`
- Access: Can only view and manage the specified client's data

**API Key Requirements:**
- Minimum 32 characters
- Only alphanumeric characters, hyphens, and underscores allowed
- Case-sensitive
- For client keys: client name must not contain hyphens (use underscores instead)

### Access Control Behavior

**Admin API Keys:**
- Can access all clients and environments
- Web UI shows full client/environment selector
- Can use any client name in API requests
- Can access all badge endpoints

**Standard API Keys:**
- Restricted to the specified client only
- Web UI auto-selects the client and shows environment selector only
- API requests are validated - can only access the authenticated client's data
- Badge URLs must match the authenticated client
- Attempting to access other clients returns `403 Forbidden`


### Authentication Methods

**1. HTTP Headers (recommended for API clients):**
```bash
# Admin API key - access all clients
curl -H "Authorization: Bearer admin-master-key-12345678901234567890" \
     "http://localhost:8080/api/releases/current?client_name=any-client&env_name=any-env"

# Client API key - restricted access
curl -H "X-API-Key: client1-authkey12345678901234567890" \
     "http://localhost:8080/api/releases/current?client_name=client1&env_name=production"
```

**2. Query Parameters (for web UI access):**
```bash
# Admin access - shows client/environment selector
http://localhost:8080/?apikey=admin-master-key-12345678901234567890

# Client-specific access - auto-selects client, shows environment selector
http://localhost:8080/?apikey=client1-authkey12345678901234567890

```

**3. Badge URLs (URL-based authentication):**
```bash
# Admin API key can access any client's badges
http://localhost:8080/badges/admin-master-key-12345678901234567890/any-client/production/deployment/app/container

# Client API key can only access its own client's badges
http://localhost:8080/badges/client1-authkey12345678901234567890/client1/production/deployment/app/container
```


### Security Notes

- **HTTPS Recommended**: Always use HTTPS in production to protect API keys in transit
- **Query Parameter Logging**: API keys in URLs may appear in access logs - use headers when possible
- **Key Rotation**: Multiple API keys are supported for zero-downtime rotation
- **Client Isolation**: Standard API keys provide secure client data isolation
- **No Authentication**: If `API_KEYS` is not set, authentication is disabled entirely


## RBAC Requirements

The application requires the following Kubernetes permissions:

```yaml
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets", "daemonsets", "replicasets"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list"]
```
## Database Schema

The SQLite database stores release information with the following schema:

```sql
CREATE TABLE releases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    namespace TEXT NOT NULL,
    workload_name TEXT NOT NULL,
    workload_type TEXT NOT NULL,
    container_name TEXT NOT NULL,
    image_repo TEXT NOT NULL,
    image_name TEXT NOT NULL,
    image_tag TEXT NOT NULL,
    client_name TEXT NOT NULL,
    env_name TEXT NOT NULL,
    first_seen DATETIME NOT NULL,
    last_seen DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(namespace, workload_name, container_name, client_name, env_name, image_repo, image_name, image_tag)
);
```
## Web Interface

### Dashboard
- **Hierarchical View**: Namespace → Workload Type → Workload Name → Container → Image Details
- **Search**: Full-text search across all fields
- **Real-time Data**: Live collection and refresh capabilities
- **Navigation**: Direct links to timeline view for each component

### Timeline
- **Component Selection**: Dropdown selectors for namespace/workload/container
- **Visual Timeline**: Chronological view of releases with timestamps
- **Release Details**: Complete image information for each release
- **URL Parameters**: Direct linking to specific component timelines

### Badges
- **Viewer**: Interactive badge generator with API key authentication
- **Authentication**: URL-based API key authentication for badge requests
- **Markdown**: Copyable markdown for README embedding

### Badge Summary
- **Interactive Table**: Select client, environments, and workloads
- **Badge Generation**: Real-time badge generation for selected components
- **Export**: Copyable markdown and HTML for embedding in READMEs

## Logging
- Structured logging for collection activities
- Error tracking and reporting
- Request logging for API endpoints

## Security

### Container Security
- Non-root user execution
- Read-only root filesystem
- Minimal Alpine base image
- Security context with dropped capabilities

### RBAC
- Least-privilege access to Kubernetes resources
- Namespace-scoped permissions where possible
- Service account isolation


## Documentation

For detailed guides on specific features and configurations:

- **[API Endpoints Documentation](docs/API_ENDPOINTS_GUIDE.md)** - REST API documentation with authentication details
- **[Comparison Page Guide](docs/COMPARISON_GUIDE.md)** - Understanding the environment comparison page
- **[Master Mode Guide](docs/MASTER_MODE_GUIDE.md)** - Setting up master instances for multi-cluster deployments
- **[Proxy Support Guide](docs/PROXY_SUPPORT_GUIDE.md)** - Configure HTTP/HTTPS proxy for sync and ping operations
- **[Ping System Guide](docs/PING_SYSTEM_GUIDE.md)** - Health monitoring and slave connectivity tracking
- **[API Key Client Access Control](docs/api-key-client-access-control-implementation.md)** - Detailed API authentication implementation

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request


