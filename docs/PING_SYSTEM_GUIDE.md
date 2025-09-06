# Health Monitoring Ping System (Master Mode)

## Overview

The Release Tracker includes a comprehensive health monitoring system that tracks the connectivity and status of slave instances from the master dashboard. This system provides real-time visual indicators of slave health and connectivity.

## Features

### 🔄 **Automatic Health Pings**
- **5-minute intervals**: Each slave sends heartbeat pings every 5 minutes
- **Retry logic**: Failed pings are retried with exponential backoff
- **Independent operation**: Works separately from the release sync mechanism

### 📊 **Master-side Monitoring**
- **Database tracking**: Stores ping history for each client/environment
- **Status calculation**: Real-time status based on last ping time
- **API integration**: Ping status included in client/environment data

### 🎨 **Visual Dashboard Indicators**
- **Client-level status**: Overall health indicator for each client
- **Environment-level status**: Individual status dots for each environment
- **Color coding**: Intuitive green/yellow/red/gray status system
- **Tooltips**: Detailed status information on hover

## Status Levels

### 🟢 **Online** (Green)
- Last ping received within **10 minutes**
- Slave is healthy and communicating normally

### 🟡 **Warning** (Yellow)
- Last ping received **10-15 minutes** ago
- Potential connectivity issues or slave problems

### 🔴 **Offline** (Red)
- Last ping received **more than 15 minutes** ago
- Slave is likely down or unreachable

### ⚪ **Never** (Gray)
- No ping ever received from this client/environment
- New or unconfigured slave instance

### ⚫ **Unknown** (Dark Gray)
- Error retrieving ping status
- Database or system issues

## Implementation Details

### **Database Schema**
```sql
CREATE TABLE slave_pings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    client_name TEXT NOT NULL,
    env_name TEXT NOT NULL,
    last_ping_time DATETIME NOT NULL,
    status TEXT NOT NULL DEFAULT 'online',
    slave_version TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(client_name, env_name)
);
```

### **API Endpoints**

#### `POST /api/ping`
Receives health pings from slave instances.

**Request Body:**
```json
{
  "client_name": "client1",
  "env_name": "production",
  "slave_version": "v1.0.0",
  "timestamp": "2025-01-09T18:06:42Z"
}
```

**Response:**
```json
{
  "status": "ok",
  "timestamp": "2025-01-09T18:06:42Z"
}
```

#### `GET /api/clients-environments` (Enhanced)
Now includes ping status for each client/environment.

**Response:**
```json
{
  "clients_environments": {
    "client1": ["dev", "prod"],
    "client2": ["staging"]
  },
  "ping_statuses": {
    "client1": {
      "dev": {
        "status": "online",
        "last_ping": "2025-01-09T18:01:42Z"
      },
      "prod": {
        "status": "warning",
        "last_ping": "2025-01-09T17:55:42Z"
      }
    }
  },
  "statistics": {
    "total_clients": 2,
    "total_environments": 3,
    "total_releases": 42
  }
}
```

## Configuration

### **Slave Configuration**
```bash
# Required for ping system
MODE=slave
MASTER_URL=http://master-instance:8080
MASTER_API_KEY=your-master-api-key1
CLIENT_NAME=your-client-name
ENV_NAME=your-environment-name
```

### **Master Configuration**
```bash
# Master mode
MODE=master
PORT=8080

# API keys for slave authentication
API_KEYS=your-master-api-key1,key2,key3
```

## Troubleshooting

### **Slave Not Pinging**
1. Check `MASTER_URL` configuration
2. Verify `MASTER_API_KEY` matches master configuration
3. Ensure network connectivity to master
4. Check slave logs for ping errors

### **Status Shows "Never"**
1. Verify slave is running and configured correctly
2. Check if `CLIENT_NAME` and `ENV_NAME` match expected values
3. Ensure master is receiving pings (check master logs)

### **Status Shows "Unknown"**
1. Check master database connectivity
2. Verify ping table exists and is accessible
3. Review master logs for database errors

### **Visual Indicators Not Updating**
1. Check browser console for JavaScript errors
2. Verify `/api/clients-environments` endpoint is accessible
3. Ensure master mode is properly configured

## Technical Considerations

- **Lightweight**: Ping system has minimal performance impact
- **Resilient**: Handles network failures gracefully with retry logic
- **Scalable**: Efficiently handles many slave instances
- **Independent**: Operates separately from release sync for reliability
- **Secure**: Uses same authentication as release sync system
