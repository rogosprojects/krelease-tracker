# Environment Comparison Page

The environment comparison page provides a bird's-eye view of ALL components across ALL environments for a selected client.

The comparison uses color-coding to show version relationships without implying which version is "correct" or "wrong". The system simply identifies which environments have matching vs. different releases, allowing teams to make informed decisions based on their deployment strategy.

## **Access Methods**
- **From Dashboard**: Click the 🔄 Compare button (appears once per client)
- **Direct URL**: `compare.html?client={client-name}&apikey={api-key}`

## **Grid Layout**
- **Rows**: Each unique component (namespace/workload/container)
- **Columns**: Each environment for the selected client
- **Cells**: Current deployment information with color-coded visual indicators

## **Component Information**
Each row displays:
- **Namespace**: Kubernetes namespace (bold, primary text)
- **Workload**: Deployment/StatefulSet/DaemonSet name (secondary text)
- **Container**: Container name (monospace, tertiary text)

## **Environment Data**
Each cell shows:
- **Image Tag**: Current version tag
- **Image SHA**: Truncated SHA with full version in tooltip
- **Background Color**: Visual indicator showing version relationships

## Visual Indicators

### **Color-Based Comparison**
- **Green Background**: Components with matching versions (same image_tag and image_sha)
- **Yellow Background**: Components with different versions from other environments
- **Gray Background**: Components not deployed in this environment


---

## Use Cases

### For DevOps Teams

#### **Timeline Page - Deep Dive Analysis**
- **Deployment Verification**: Ensure specific components are deployed consistently
- **Rollback Planning**: Identify which environments need updates
- **Release Coordination**: Verify deployment status before promoting to production

#### **Environment Comparison - Overview Analysis**
- **Environment Drift Detection**: Quickly identify version inconsistencies
- **Release Readiness**: Ensure all components are aligned before releases
- **Compliance Checking**: Verify that production matches approved versions
- **Deployment Planning**: Identify which components need updates across environments

### For Development Teams

#### **Release Tracking**
- **Feature Deployment**: Track feature rollout across environments
- **Bug Fix Verification**: Confirm fixes are deployed everywhere
- **Version Alignment**: Ensure development, staging, and production are synchronized

#### **Troubleshooting**
- **Environment Comparison**: Identify configuration differences causing issues
- **Deployment Status**: Verify successful deployments across environments
- **Rollback Coordination**: Identify which environments need rollback

---

## Navigation Flow

```
Dashboard (index.html)
    ├── History Button → Timeline Page (timeline.html)
    │   └── Cross-environment comparison for single component
    │
    └── 🔄 Compare Button → Environment Comparison (compare.html)
        └── All components across all environments
```

This comprehensive web interface provides multiple perspectives on deployment data, enabling both detailed component analysis and high-level environment overview for effective DevOps workflows.
