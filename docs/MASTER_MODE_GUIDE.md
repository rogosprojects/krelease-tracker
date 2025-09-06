# Master Mode Multi-Client UI Guide

## Overview

The Release Tracker supports a multi-client, multi-environment data visualization system when running in **Master Mode**. This allows you to aggregate and view release data from multiple slave instances, each representing different clients and environments.

## Benefits

1. **Centralized Monitoring**: View all client deployments from one dashboard
2. **Environment Isolation**: Clear separation between dev, staging, and production
3. **Multi-Tenant Support**: Each client maintains their own data and environments
5. **Scalable Architecture**: Easy to add new clients and environments

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Slave Mode    │    │   Slave Mode    │    │   Master Mode   │
│   Client A      │───▶│   Client B      │───▶│   Aggregator    │
│   - dev         │    │   - staging     │    │   - All Clients │
│   - prod        │    │   - prod        │    │   - All Envs    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
        │                     |                     |
        ▼                     ▼                     ▼
  ┌──────────────┐        ┌──────────────┐    ┌───────────────┐
  │   SQLite DB  │        │   SQLite DB  │    │  SQLite DB    │
  │              |        |              |    | (all clients) │
  └──────────────┘        └──────────────┘    └───────────────┘
```
## Features

### 🎯 Control Panel
- **Aggregate Statistics**: Real-time overview of total clients, environments, and releases
- **Interactive Client/Environment Grid**: Card-based selector with hover effects

### 📊 Smart Data Filtering
- **Client Selection**: Click on any client card to view their environments
- **Environment Selection**: Click on environment tags to filter releases
- **Real-time Updates**: Instant filtering without page reloads
- **Clear Selection**: Easy reset to return to overview state

### 🎨 Modern UX Design
- **Gradient Background**: Beautiful purple gradient control panel
- **Glass Morphism**: Translucent cards with backdrop blur effects
- **Smooth Animations**: Hover effects and transitions throughout
- **Responsive Design**: Works on desktop, tablet, and mobile devices

## How to Use

### 1. **Starting in Master Mode**
```bash
export MODE=master
export PORT=8080
export DATABASE_PATH=/data/releases.db
./release-tracker
```

### 2. **Control Panel Overview**
When you open the dashboard in master mode, you'll see:
- **Statistics Cards**: Total clients, environments, and releases
- **Client Grid**: Interactive cards showing each client and their environments

### 3. **Selecting Data to View**
1. **Browse Clients**: Each client is displayed as a card with their available environments
2. **Select Environment**: Click on an environment tag (e.g., "prod", "staging", "dev")
3. **View Releases**: The releases table will automatically filter to show only that client/environment
4. **Clear Selection**: Use the "Clear Selection" button to return to the overview

### 4. **Visual Indicators**
- **Selected Client**: Card gets a green border and glow effect
- **Selected Environment**: Environment tag turns green with shadow
- **Current Selection**: Shows "Viewing: ClientName - Environment" when active
- **No Selection State**: Helpful message when no client/environment is selected

## API Endpoints

### New Endpoints for Master Mode

#### `GET /api/clients-environments`
Returns available clients and environments with statistics:
```json
{
  "clients_environments": {
    "client1": ["dev", "staging", "prod"],
    "client2": ["dev", "prod"]
  },
  "statistics": {
    "total_clients": 2,
    "total_environments": 5,
    "total_releases": 42
  },
  "timestamp": "2025-01-09T01:40:00Z"
}
```

#### `GET /api/releases/current?client_name=X&env_name=Y`
Returns filtered releases for specific client and environment:
```bash
# Get releases for client1's production environment
GET /api/releases/current?client_name=client1&env_name=prod
```

#### `GET /api/config`
Returns application configuration including mode:
```json
{
  "mode": "master",
  "env_name": "unknown",
  "client_name": "unknown",
  "timestamp": "2025-01-09T01:40:00Z"
}
```

## Default Behavior

### ✅ **Master Mode**
- Control panel for client/environment selection is automatically displayed
- Collect button is hidden (collection disabled)
- No releases shown until client/environment is selected
- Manual collection API remains available for slaves

### ✅ **Slave Mode**
- Normal dashboard view (no control panel)
- Collect button is visible and functional
- Automatic collection enabled
- Releases stored in both tables (historical + queue)

## Environment Variables

```bash
# Application mode
MODE=master                    # or "slave" (default)

# Master mode specific
MASTER_URL=http://master:8080  # URL of master instance (slave mode)
MASTER_API_KEY=your-api-key    # API key for master auth (slave mode)
PROXY_URL=http://proxy:8080    # HTTP/HTTPS proxy for sync requests (slave mode, optional)
TLS_INSECURE=false             # Skip TLS certificate verification (slave mode, optional)

# General configuration
PORT=8080
DATABASE_PATH=/data/releases.db
CLIENT_NAME=your-client-name
ENV_NAME=your-environment
```

## Troubleshooting

### No Data Showing
- Ensure slaves are properly configured with `MASTER_URL` and `MASTER_API_KEY`
- Check that slaves are successfully syncing to master
- Verify client/environment selection is active

### Control Panel Not Appearing
- Confirm `MODE=master` environment variable is set
- Check browser console for JavaScript errors

### API Authentication Issues
- Verify `MASTER_API_KEY` matches between slave and master
- Check API key configuration in master instance
- Review server logs for authentication errors
