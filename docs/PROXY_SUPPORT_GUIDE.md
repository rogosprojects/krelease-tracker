# Proxy Support Guide

## Overview

The Release Tracker includes comprehensive proxy support for both sync and ping operations in slave mode. This allows the application to work in corporate environments where internet access is restricted through proxy servers.

## Features

### 🌐 **HTTP/HTTPS Proxy Support**
- **Sync operations**: All release synchronization requests to master can be routed through a proxy
- **Ping operations**: In master/slave configuration, health monitoring pings to master can be routed through a proxy
- **TLS flexibility**: Support for insecure TLS connections when working with internal proxies

### 🔧 **Configuration Options**
- **PROXY_URL**: HTTP/HTTPS proxy server URL
- **TLS_INSECURE**: Disable TLS certificate verification for internal proxies

## Configuration

### Environment Variables

```bash
# Proxy configuration
PROXY_URL=http://proxy.company.com:8080
# TLS configuration
TLS_INSECURE=true  # Disable TLS certificate verification
```

### Slave Configuration Example

```bash
# Required slave configuration
MODE=slave
MASTER_URL=https://master-instance.company.com:8080
MASTER_API_KEY=your-master-api-key
CLIENT_NAME=production-cluster
ENV_NAME=production

# Proxy configuration
PROXY_URL=http://proxy.company.com:8080
TLS_INSECURE=false

# Optional configuration
COLLECTION_INTERVAL=5
DATABASE_PATH=/data/releases.db
NAMESPACES=default,production,staging
```

## Security Considerations

- **Credentials in URLs**: Be careful when including authentication in proxy URLs
- **TLS verification**: Only disable TLS verification (`TLS_INSECURE=true`) in trusted environments
- **Proxy logs**: Be aware that proxy servers may log all traffic
- **Environment variables**: Secure environment variable storage for proxy credentials

## Related Documentation

- [Master Mode Guide](MASTER_MODE_GUIDE.md) - Master instance configuration
- [Ping System Guide](PING_SYSTEM_GUIDE.md) - Health monitoring system
