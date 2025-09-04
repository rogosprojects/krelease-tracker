package kubernetes

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"release-tracker/internal/database"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Client wraps the Kubernetes client
type Client struct {
	clientset  *kubernetes.Clientset
	namespaces []string
	mode       string
}

// New creates a new Kubernetes client
func New(inCluster bool, kubeconfigPath string, namespaces []string, mode string) (*Client, error) {
	var config *rest.Config
	var err error

	if inCluster {
		config, err = rest.InClusterConfig()
	} else {
		if kubeconfigPath == "" {
			if home := homedir.HomeDir(); home != "" {
				kubeconfigPath = filepath.Join(home, ".kube", "config")
			}
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &Client{
		clientset:  clientset,
		namespaces: namespaces,
		mode:       mode,
	}, nil
}

// CollectReleases discovers all workloads and their container images across monitored namespaces
func (c *Client) CollectReleases(ctx context.Context, db *database.DB) error {
	log.Printf("Starting collection across namespaces: %v", c.namespaces)

	for _, namespace := range c.namespaces {
		if err := c.collectNamespaceReleases(ctx, db, namespace); err != nil {
			log.Printf("Error collecting releases from namespace %s: %v", namespace, err)
			continue
		}
	}

	// Cleanup old releases after collection
	if err := db.CleanupOldReleases(); err != nil {
		log.Printf("Error cleaning up old releases: %v", err)
	}

	log.Printf("Collection completed")
	return nil
}

// collectNamespaceReleases collects releases from a specific namespace
func (c *Client) collectNamespaceReleases(ctx context.Context, db *database.DB, namespace string) error {
	log.Printf("Collecting releases from namespace: %s", namespace)

	// Collect from Deployments
	if err := c.collectDeployments(ctx, db, namespace); err != nil {
		return fmt.Errorf("failed to collect deployments: %w", err)
	}

	// Collect from StatefulSets
	if err := c.collectStatefulSets(ctx, db, namespace); err != nil {
		return fmt.Errorf("failed to collect statefulsets: %w", err)
	}

	// Collect from DaemonSets
	if err := c.collectDaemonSets(ctx, db, namespace); err != nil {
		return fmt.Errorf("failed to collect daemonsets: %w", err)
	}

	// // Collect from ReplicaSets (standalone ones)
	// if err := c.collectReplicaSets(ctx, db, namespace); err != nil {
	// 	return fmt.Errorf("failed to collect replicasets: %w", err)
	// }

	return nil
}

// collectDeployments collects container images from Deployments
func (c *Client) collectDeployments(ctx context.Context, db *database.DB, namespace string) error {
	deployments, err := c.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, deployment := range deployments.Items {
		if err := c.processWorkload(ctx, db, namespace, deployment.Name, "Deployment", deployment.Spec.Template.Spec); err != nil {
			log.Printf("Error processing deployment %s/%s: %v", namespace, deployment.Name, err)
		}
	}

	return nil
}

// collectStatefulSets collects container images from StatefulSets
func (c *Client) collectStatefulSets(ctx context.Context, db *database.DB, namespace string) error {
	statefulSets, err := c.clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, statefulSet := range statefulSets.Items {
		if err := c.processWorkload(ctx, db, namespace, statefulSet.Name, "StatefulSet", statefulSet.Spec.Template.Spec); err != nil {
			log.Printf("Error processing statefulset %s/%s: %v", namespace, statefulSet.Name, err)
		}
	}

	return nil
}

// collectDaemonSets collects container images from DaemonSets
func (c *Client) collectDaemonSets(ctx context.Context, db *database.DB, namespace string) error {
	daemonSets, err := c.clientset.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, daemonSet := range daemonSets.Items {
		if err := c.processWorkload(ctx, db, namespace, daemonSet.Name, "DaemonSet", daemonSet.Spec.Template.Spec); err != nil {
			log.Printf("Error processing daemonset %s/%s: %v", namespace, daemonSet.Name, err)
		}
	}

	return nil
}

// // collectReplicaSets collects container images from standalone ReplicaSets
// func (c *Client) collectReplicaSets(ctx context.Context, db *database.DB, namespace string) error {
// 	replicaSets, err := c.clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
// 	if err != nil {
// 		return err
// 	}

// 	for _, replicaSet := range replicaSets.Items {
// 		// Skip ReplicaSets owned by Deployments
// 		if len(replicaSet.OwnerReferences) > 0 {
// 			for _, owner := range replicaSet.OwnerReferences {
// 				if owner.Kind == "Deployment" {
// 					continue
// 				}
// 			}
// 		}

// 		if err := c.processWorkload(ctx, db, namespace, replicaSet.Name, "ReplicaSet", replicaSet.Spec.Template.Spec); err != nil {
// 			log.Printf("Error processing replicaset %s/%s: %v", namespace, replicaSet.Name, err)
// 		}
// 	}

// 	return nil
// }

// processWorkload processes a workload's pod spec and extracts container information
func (c *Client) processWorkload(ctx context.Context, db *database.DB, namespace, workloadName, workloadType string, podSpec corev1.PodSpec) error {
	now := time.Now()

	// Process all containers (including init containers)
	//allContainers := append(podSpec.Containers, podSpec.InitContainers...)

	// For now, only process app containers
	allContainers := podSpec.Containers

	// Get client and environment names from environment variables
	clientName := os.Getenv("CLIENT_NAME")
	if clientName == "" {
		log.Printf("Error: CLIENT_NAME environment variable not set.")
		return fmt.Errorf("CLIENT_NAME environment variable not set")
	}
	envName := os.Getenv("ENV_NAME")
	if envName == "" {
		log.Printf("Error: ENV_NAME environment variable not set.")
		return fmt.Errorf("ENV_NAME environment variable not set")
	}

	for _, container := range allContainers {
		repo, name, tag := database.ParseImagePath(container.Image)

		// Get the actual image SHA256 from running pods
		imageSHA, err := c.getImageSHAFromPods(ctx, namespace, workloadName, workloadType, container.Name)
		if err != nil {
			log.Printf("Error: Could not get image SHA for %s/%s/%s: %v", namespace, workloadName, container.Name, err)
			// Do not Continue with empty SHA
			// Skip this container
			continue
		}

		// Create release object for historical data
		release := &database.Release{
			Namespace:     namespace,
			WorkloadName:  workloadName,
			WorkloadType:  workloadType,
			ContainerName: container.Name,
			ImageRepo:     repo,
			ImageName:     name,
			ImageTag:      tag,
			ImageSHA:      imageSHA,
			ClientName:    clientName,
			EnvName:       envName,
			FirstSeen:     now,
			LastSeen:      now,
		}

		// Always store in releases table for historical data
		if err := db.UpsertRelease(release); err != nil {
			return fmt.Errorf("failed to upsert release: %w", err)
		}

		// In slave mode, also store in pending_releases table as queue
		if c.mode == "slave" {
			pendingRelease := &database.PendingRelease{
				Namespace:     namespace,
				WorkloadName:  workloadName,
				WorkloadType:  workloadType,
				ContainerName: container.Name,
				ImageRepo:     repo,
				ImageName:     name,
				ImageTag:      tag,
				ImageSHA:      imageSHA,
				ClientName:    clientName,
				EnvName:       envName,
				FirstSeen:     now,
				LastSeen:      now,
			}

			if err := db.UpsertPendingRelease(pendingRelease); err != nil {
				return fmt.Errorf("failed to upsert pending release: %w", err)
			}
		}
	}

	return nil
}

// getImageSHAFromPods queries running pods to get the actual image SHA256 digest for a container
func (c *Client) getImageSHAFromPods(ctx context.Context, namespace, workloadName, workloadType, containerName string) (string, error) {
	// Create label selector based on workload type
	var labelSelector string
	switch workloadType {
	case "Deployment":
		labelSelector = fmt.Sprintf("app=%s", workloadName)
	case "StatefulSet":
		labelSelector = fmt.Sprintf("app=%s", workloadName)
	case "DaemonSet":
		labelSelector = fmt.Sprintf("app=%s", workloadName)
	default:
		// Try common label patterns
		labelSelector = fmt.Sprintf("app=%s", workloadName)
	}

	// Query pods with the label selector
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list pods: %w", err)
	}

	// If no pods found with app label, try alternative selectors
	if len(pods.Items) == 0 {
		// Try with workload name as label value
		labelSelector = fmt.Sprintf("app.kubernetes.io/name=%s", workloadName)
		pods, err = c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return "", fmt.Errorf("failed to list pods with alternative selector: %w", err)
		}
	}

	// If still no pods found, try without label selector but filter by owner reference
	if len(pods.Items) == 0 {
		allPods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to list all pods: %w", err)
		}

		// Filter pods by owner reference
		for _, pod := range allPods.Items {
			for _, ownerRef := range pod.OwnerReferences {
				if ownerRef.Kind == workloadType && ownerRef.Name == workloadName {
					pods.Items = append(pods.Items, pod)
					break
				}
				// Also check for ReplicaSet ownership (for Deployments)
				if ownerRef.Kind == "ReplicaSet" && workloadType == "Deployment" {
					// Get the ReplicaSet to check its owner
					rs, err := c.clientset.AppsV1().ReplicaSets(namespace).Get(ctx, ownerRef.Name, metav1.GetOptions{})
					if err == nil {
						for _, rsOwnerRef := range rs.OwnerReferences {
							if rsOwnerRef.Kind == "Deployment" && rsOwnerRef.Name == workloadName {
								pods.Items = append(pods.Items, pod)
								break
							}
						}
					}
				}
			}
		}
	}

	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no running pods found for %s/%s", workloadType, workloadName)
	}

	// Look for a running pod with the specified container
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		// Check container statuses for the image ID
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.Name == containerName && containerStatus.Ready {
				// Extract SHA256 from ImageID
				// ImageID format is typically: docker-pullable://registry/image@sha256:digest
				// or docker://sha256:digest
				imageID := containerStatus.ImageID
				if imageID == "" {
					continue
				}

				// Extract SHA256 digest from ImageID
				sha256 := extractSHA256FromImageID(imageID)
				if sha256 != "" {
					return sha256, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no ready container %s found in running pods for %s/%s", containerName, workloadType, workloadName)
}

// extractSHA256FromImageID extracts the SHA256 digest from a Kubernetes ImageID
func extractSHA256FromImageID(imageID string) string {
	// ImageID can be in various formats:
	// docker-pullable://registry/image@sha256:digest
	// docker://sha256:digest
	// registry/image@sha256:digest
	// sha256:digest

	// Look for sha256: pattern
	if idx := strings.Index(imageID, "sha256:"); idx != -1 {
		sha256Part := imageID[idx+7:] // Skip "sha256:"
		// Take only the hex digest part (64 characters)
		if len(sha256Part) >= 64 {
			return sha256Part[:64]
		}
		return sha256Part
	}

	return ""
}
