# Release Tracker API Documentation

## Architecture Overview

Release Tracker supports two deployment configurations with different API endpoint availability:

### Master-Only Instance
A standalone instance that monitors a single Kubernetes cluster:
- **Available Endpoints**: All collection, release, health, and badge endpoints
- **Authentication**: Standard API key authentication for all `/api/*` endpoints
- **Use Case**: Single cluster monitoring, development environments

### Master/Slave Configuration
A distributed setup with one master coordinating multiple slave instances:

**Master Instance:**
- **Available Endpoints**: All endpoints including master-specific coordination endpoints
- **Authentication**: Admin API keys for full access, client-specific keys for filtered access
- **Use Case**: Multi-cluster monitoring, centralized dashboard

**Slave Instances:**
- **Available Endpoints**: Collection endpoints for local cluster monitoring
- **Authentication**: Uses master API key for synchronization
- **Use Case**: Remote cluster monitoring that reports to master

---

## Authentication

### API Key Types

**Admin API Keys** (Master mode):
- Format: `admin-key-here` (no hyphens in client portion)
- Access: Full access to all clients and environments
- Usage: Master instance administration

**Client-Specific API Keys** (Master mode):
- Format: `client-name-auth-token` (single hyphen separator)
- Access: Limited to specific client's data
- Usage: Filtered access for specific clients

**Standard API Keys** (Master-only mode):
- Format: Any valid API key format
- Access: Full access to instance data
- Usage: Single instance authentication

### Authentication Headers
```bash
Authorization: Bearer your-api-key-here
```

---

## Release Collection
- `POST /api/collect` - Trigger immediate collection of cluster state
- `PUT /api/collect/{namespace}/{workload-kind}/{workload-name}/{container}` - Manually add a new workload release

#### Manual Collection Endpoint

The manual collection endpoint allows you to manually register a new release for a specific workload and container. This is useful for tracking releases that are deployed outside of the monitored Kubernetes clusters or for historical data entry.

**Endpoint:**
```
PUT /api/collect/{namespace}/{workload-kind}/{workload-name}/{container}
```

**Authentication:** Required (Bearer token)

**Path Parameters:**
- `namespace`: Kubernetes namespace (e.g., "production", "staging")
- `workload-kind`: Type of workload (e.g., "Deployment", "StatefulSet", "DaemonSet")
- `workload-name`: Name of the workload (e.g., "web-server", "database")
- `container`: Container name within the workload (e.g., "app", "nginx", "postgres")

**Request Body:**
- `image_tag` (required): Image tag (e.g., "1.21.0", "v1.2.3", "latest")
- `image_sha` (required): SHA256 digest of the container image for accurate tracking
- `image_repo` (optional): Image repository (e.g., "docker.io", "gcr.io/myproject")
- `image_name` (optional): Image name (e.g., "nginx", "myapp")
- `client_name` (optional): Client/cluster name. Defaults to configured client name if not provided
- `env_name` (optional): Environment name. Defaults to configured environment name if not provided
- `released_at` (optional): ISO 8601 timestamp when the release was deployed. Defaults to current time if not provided

**Example Request:**
```bash
curl -X PUT "https://release-tracker.example.com/api/collect/production/Deployment/web-app/nginx" \
  -H "Authorization: Bearer your-api-key-here" \
  -H "Content-Type: application/json" \
  -d '{
    "image_tag": "1.21.0",
    "image_sha": "sha256:abc123def456789012345678901234567890123456789012345678901234567890",
    "image_repo": "docker.io",
    "image_name": "nginx",
    "released_at": "2023-12-01T10:30:00Z"
  }'
```

**Success Response (200 OK):**
```json
{
  "status": "success",
  "message": "Release collected successfully",
  "component": {
    "namespace": "production",
    "workload_kind": "Deployment",
    "workload_name": "web-app",
    "container_name": "nginx"
  },
  "release": {
    "version": "1.21.0",
    "image_repo": "docker.io",
    "image_name": "nginx",
    "image_tag": "1.21.0",
    "image_sha": "sha256:abc123def456789012345678901234567890123456789012345678901234567890",
    "released_at": "2023-12-01T10:30:00Z"
  },
  "timestamp": "2023-12-01T10:35:22Z"
}
```

**Error Responses:**
- `400 Bad Request`: Missing required fields or invalid JSON
- `401 Unauthorized`: Invalid or missing API key
- `500 Internal Server Error`: Database or server error

### Current Releases

#### Get Current Releases
```
GET /api/releases/current?client_name={client}&env_name={environment}
```

**Authentication:** Required (Bearer token)

**Query Parameters:**
- `client_name` (required): Client/cluster name to filter releases
- `env_name` (required): Environment name to filter releases

**Access Control:**
- **Admin API keys**: Can access any client/environment combination
- **Client-specific API keys**: Can only access their own client's data
- **Master-only mode**: No filtering required, returns all releases

**Example Request:**
```bash
curl -X GET "https://release-tracker.example.com/api/releases/current?client_name=production-cluster&env_name=prod" \
  -H "Authorization: Bearer your-api-key-here"
```

**Success Response (200 OK):**
```json
{
  "namespaces": {
    "default": [
      {
        "namespace": "default",
        "workload_name": "web-app",
        "workload_type": "Deployment",
        "container_name": "nginx",
        "image_repo": "docker.io",
        "image_name": "nginx",
        "image_tag": "1.21.0",
        "image_sha": "sha256:abc123...",
        "client_name": "production-cluster",
        "env_name": "prod",
        "first_seen": "2023-12-01T10:30:00Z",
        "last_seen": "2023-12-01T15:45:00Z"
      }
    ]
  },
  "ordered_namespaces": ["default"],
  "total": 1,
  "timestamp": "2023-12-01T15:45:00Z"
}
```

**Error Responses:**
- `400 Bad Request`: Missing required query parameters
- `401 Unauthorized`: Invalid or missing API key
- `403 Forbidden`: API key not authorized for requested client
- `500 Internal Server Error`: Database or server error

### Release History

#### Get Release History
```
GET /api/releases/history/{client}/{env}/{namespace}/{workload}/{container}
```

**Authentication:** Required (Bearer token)

**Path Parameters:**
- `client`: Client/cluster name
- `env`: Environment name
- `namespace`: Kubernetes namespace
- `workload`: Workload name
- `container`: Container name

**Access Control:**
- **Admin API keys**: Can access any client/environment combination
- **Client-specific API keys**: Can only access their own client's data
- **Master-only mode**: Client/env parameters still required but no filtering applied

**Example Request:**
```bash
curl -X GET "https://release-tracker.example.com/api/releases/history/production-cluster/prod/default/web-app/nginx" \
  -H "Authorization: Bearer your-api-key-here"
```

**Success Response (200 OK):**
```json
{
  "component": {
    "client_name": "production-cluster",
    "env_name": "prod",
    "namespace": "default",
    "workload_name": "web-app",
    "container_name": "nginx"
  },
  "releases": [
    {
      "image_tag": "1.21.0",
      "image_sha": "sha256:abc123...",
      "first_seen": "2023-12-01T10:30:00Z",
      "last_seen": "2023-12-01T15:45:00Z"
    },
    {
      "image_tag": "1.20.0",
      "image_sha": "sha256:def456...",
      "first_seen": "2023-11-15T09:00:00Z",
      "last_seen": "2023-12-01T10:29:59Z"
    }
  ],
  "total": 2,
  "timestamp": "2023-12-01T15:45:00Z"
}
```

**Error Responses:**
- `401 Unauthorized`: Invalid or missing API key
- `403 Forbidden`: API key not authorized for requested client
- `404 Not Found`: Component not found
- `500 Internal Server Error`: Database or server error

---

## Master-Mode Specific Endpoints

The following endpoints are only available when running in master mode (`MODE=master`):

### Clients and Environments

#### Get Available Clients and Environments
```
GET /api/clients-environments
```

**Authentication:** Required (Bearer token)

**Description:** Returns all available client/environment combinations with statistics and ping status.

**Example Request:**
```bash
curl -X GET "https://release-tracker.example.com/api/clients-environments" \
  -H "Authorization: Bearer your-api-key-here"
```

**Success Response (200 OK):**
```json
{
  "clients_environments": {
    "production-cluster": ["prod", "staging"],
    "dev-cluster": ["dev", "test"]
  },
  "ping_statuses": {
    "production-cluster": {
      "prod": {
        "status": "online",
        "last_ping": "2023-12-01T15:40:00Z"
      },
      "staging": {
        "status": "offline"
      }
    }
  },
  "statistics": {
    "total_clients": 2,
    "total_environments": 4,
    "total_releases": 42
  },
  "timestamp": "2023-12-01T15:45:00Z"
}
```

### Slave Ping

#### Receive Slave Health Ping
```
POST /api/ping
```

**Authentication:** Required (Bearer token)

**Description:** Receives health pings from slave instances for connectivity monitoring.

**Request Body:**
- `client_name` (required): Client/cluster name
- `env_name` (required): Environment name
- `slave_version` (optional): Version of the slave instance
- `timestamp` (optional): Ping timestamp

**Example Request:**
```bash
curl -X POST "https://release-tracker.example.com/api/ping" \
  -H "Authorization: Bearer your-api-key-here" \
  -H "Content-Type: application/json" \
  -d '{
    "client_name": "production-cluster",
    "env_name": "prod",
    "slave_version": "v1.0.0",
    "timestamp": "2023-12-01T15:45:00Z"
  }'
```

**Success Response (200 OK):**
```json
{
  "status": "ok",
  "timestamp": "2023-12-01T15:45:00Z"
}
```

### Application Configuration

#### Get Application Configuration
```
GET /api/config
```

**Authentication:** Required (Bearer token)

**Description:** Returns application configuration including mode and client information.

**Example Request:**
```bash
curl -X GET "https://release-tracker.example.com/api/config" \
  -H "Authorization: Bearer your-api-key-here"
```

**Success Response (200 OK):**
```json
{
  "mode": "master",
  "env_name": "unknown",
  "client_name": "unknown",
  "api_key_type": {
    "is_admin": true,
    "authenticated_client": ""
  }
}
```

---

## General Endpoints

### Health Check

#### Application Health Status
```
GET /health
```

**Authentication:** None required

**Description:** Returns the health status of the application and database connectivity.

**Example Request:**
```bash
curl -X GET "https://release-tracker.example.com/health"
```

**Success Response (200 OK):**
```json
{
  "status": "healthy",
  "timestamp": "2023-12-01T15:45:00Z",
  "version": "1.0.0"
}
```

**Error Response (503 Service Unavailable):**
```json
{
  "status": "unhealthy",
  "database_error": "connection failed",
  "timestamp": "2023-12-01T15:45:00Z",
  "version": "1.0.0"
}
```

### Release Badges

#### Badge Endpoint
```
GET /badges/{api-key}/{client}/{env}/{workload-kind}/{workload-name}/{container}
```

**Authentication:** URL-based API key (no Bearer header required)

**Path Parameters:**
- `api-key`: API key for authentication (visible in URL)
- `client`: Client/cluster name
- `env`: Environment name
- `workload-kind`: Type of workload (e.g., "Deployment", "StatefulSet", "DaemonSet")
- `workload-name`: Name of the workload
- `container`: Container name within the workload

**Response:** SVG badge image displaying environment name and current release version

**Access Control:**
- **Admin API keys**: Can access any client/environment combination
- **Client-specific API keys**: Can only access their own client's data
- **Master-only mode**: Client/env parameters still required in URL

**Badge Examples:**
```
GET /badges/your-api-key-here/production-cluster/prod/Deployment/my-app/web
GET /badges/your-api-key-here/staging-cluster/staging/StatefulSet/database/postgres
GET /badges/your-api-key-here/dev-cluster/dev/DaemonSet/logging/fluentd
```

**Badge States:**
- 🟢 **Green**: Successfully deployed with version
- 🔴 **Red**: Query error, invalid request, or authentication failure
- ⚪ **Gray**: No deployment found
- 🟡 **Yellow**: Multiple deployments found in different namespaces

**Usage in README:**
```markdown
![Release Badge](https://your-release-tracker.example.com/badges/your-api-key-here/production-cluster/prod/Deployment/my-app/web)
```

**Security Notes:**
- API keys are visible in badge URLs - use dedicated read-only keys
- Consider using client-specific API keys to limit access scope
- Badge URLs are cached by browsers and README viewers