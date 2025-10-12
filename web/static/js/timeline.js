class Timeline {
    constructor() {
        this.releases = [];
        this.components = {};
        this.selectedComponent = {
            namespace: '',
            workload: '',
            container: '',
            workloadType: ''
        };
        this.apiKey = this.extractAPIKey();
        this.basePath = this.getBasePath();
        this.config = null;
        this.selectedClient = null;
        this.selectedEnvironment = null;
        this.clientsEnvironments = null;
        this.init();
    }

    // Local storage utility functions
    saveSelectionToStorage() {
        if (this.selectedClient && this.selectedEnvironment) {
            localStorage.setItem('release-tracker-client', this.selectedClient);
            localStorage.setItem('release-tracker-environment', this.selectedEnvironment);
        }
    }

    loadSelectionFromStorage() {
        const savedClient = localStorage.getItem('release-tracker-client');
        const savedEnvironment = localStorage.getItem('release-tracker-environment');

        if (savedClient && savedEnvironment) {
            return { client: savedClient, environment: savedEnvironment };
        }
        return null;
    }

    clearStoredSelection() {
        localStorage.removeItem('release-tracker-client');
        localStorage.removeItem('release-tracker-environment');
    }

    // Get base path from current URL
    getBasePath() {
        const path = window.location.pathname;
        const segments = path.split('/').filter(s => s);

        // If we're at a specific page (dashboard.html, timeline.html), remove the filename
        if (segments.length > 0 && segments[segments.length - 1].includes('.html')) {
            segments.pop();
        }

        // Return base path or empty string
        return segments.length > 0 ? '/' + segments.join('/') : '';
    }

    async init() {
        await this.loadConfig();
        this.parseUrlParams();
        this.bindEvents();
        this.updateNavigationLinks();
        await this.loadComponents();
        await this.loadClientsEnvironments();
    }

    // Extract API key from URL query parameters
    extractAPIKey() {
        const urlParams = new URLSearchParams(window.location.search);
        return urlParams.get('apikey') || '';
    }

    // Create fetch options with API key authentication
    getFetchOptions(options = {}) {
        const fetchOptions = {
            ...options,
            headers: {
                ...options.headers
            }
        };

        // Add API key to headers if available
        if (this.apiKey) {
            fetchOptions.headers['X-API-Key'] = this.apiKey;
        }

        return fetchOptions;
    }

    // Handle authentication errors
    handleAuthError(error, response) {
        if (response && response.status === 401) {
            const message = this.apiKey
                ? 'Invalid API key. Please check your authentication credentials.'
                : 'Authentication required. Add ?apikey=your-key-here to the URL to authenticate.';
            this.showError(message);
            return true;
        }
        return false;
    }

    // Update navigation links to preserve API key and base path
    updateNavigationLinks() {
        if (this.apiKey) {
            const dashboardLink = document.querySelector('a[href="index.html"]');
            const badgeLink = document.querySelector('a[href="badges.html"]');
            const timelineLink = document.querySelector('a[href="timeline.html"]');

            if (dashboardLink) {
                dashboardLink.href = `${this.basePath}/index.html?apikey=${encodeURIComponent(this.apiKey)}`;
            }
            if (badgeLink) {
                badgeLink.href = `${this.basePath}/badges.html?apikey=${encodeURIComponent(this.apiKey)}`;
            }
            if (timelineLink) {
                timelineLink.href = `${this.basePath}/timeline.html?apikey=${encodeURIComponent(this.apiKey)}`;
            }
        }
    }

    bindEvents() {
        // Component selectors
        document.getElementById('namespaceSelect').addEventListener('change', (e) => {
            this.onNamespaceChange(e.target.value);
        });

        document.getElementById('workloadSelect').addEventListener('change', (e) => {
            this.onWorkloadChange(e.target.value);
        });

        document.getElementById('containerSelect').addEventListener('change', (e) => {
            this.onContainerChange(e.target.value);
        });

        // Action buttons
        document.getElementById('loadTimelineBtn').addEventListener('click', () => {
            this.loadTimeline();
        });

        document.getElementById('backToDashboard').addEventListener('click', () => {
            const url = this.apiKey ? `index.html?apikey=${encodeURIComponent(this.apiKey)}` : 'index.html';
            window.location.href = url;
        });

        // Badge generation button
        document.getElementById('generateBadgeBtn').addEventListener('click', () => {
            this.generateBadge();
        });

        // Error dismissal
        document.getElementById('dismissError').addEventListener('click', () => {
            this.hideError();
        });
    }

    parseUrlParams() {
        const params = new URLSearchParams(window.location.search);
        const namespace = params.get('namespace');
        const workload = params.get('workload');
        const container = params.get('container');
        const client = params.get('client');
        const env = params.get('env');

        if (namespace && workload && container && client && env) {
            this.selectedComponent = { namespace, workload, container };
            this.selectedClient = client;
            this.selectedEnvironment = env;
            // Save to local storage when URL params are provided
            this.saveSelectionToStorage();
            // Wait for components to load, then set selections
            setTimeout(() => this.setSelectionsFromUrl(), 1000);
        } else {
            console.log('No URL params found, loading from storage or config...');

            if (this.config.mode === 'slave') {
                this.selectedClient = this.config.client_name;
                this.selectedEnvironment = this.config.env_name;
            } else {
                // If no URL params, try to load from local storage
                this.loadFromStorage();
            }

            console.log('Selected client:', this.selectedClient);
            console.log('Selected environment:', this.selectedEnvironment);
        }
    }

    async loadConfig() {
        try {
            const response = await fetch(`${this.basePath}/api/config`, this.getFetchOptions());
            if (response.ok) {
                const config = await response.json();
                this.config = config;
            }
        } catch (error) {
            console.warn('Failed to load config:', error);
        }
    }

    loadFromStorage() {
        const savedSelection = this.loadSelectionFromStorage();
        if (savedSelection) {
            this.selectedClient = savedSelection.client;
            this.selectedEnvironment = savedSelection.environment;
        }
    }

    async loadClientsEnvironments() {
        try {
            const response = await fetch(`${this.basePath}/api/clients-environments`, this.getFetchOptions());
            if (response.ok) {
                const data = await response.json();
                this.clientsEnvironments = data;
            }
        } catch (error) {
            console.warn('Failed to load clients and environments:', error);
        }
    }

    setSelectionsFromUrl() {
        const { namespace, workload, container } = this.selectedComponent;

        // Set namespace
        const namespaceSelect = document.getElementById('namespaceSelect');
        namespaceSelect.value = namespace;
        this.onNamespaceChange(namespace);

        // Set workload after a short delay
        setTimeout(() => {
            const workloadSelect = document.getElementById('workloadSelect');
            workloadSelect.value = workload;
            this.onWorkloadChange(workload);

            // Set container after another short delay
            setTimeout(() => {
                const containerSelect = document.getElementById('containerSelect');
                containerSelect.value = container;
                this.onContainerChange(container);

                // Load timeline
                setTimeout(() => this.loadTimeline(), 100);
            }, 100);
        }, 100);
    }

    async loadComponents() {
        this.showLoading();
        this.hideError();

        try {
            const params = new URLSearchParams();
            params.append('client_name', this.selectedClient);
            params.append('env_name', this.selectedEnvironment);

            const url = `${this.basePath}/api/releases/current?${params}`;
            const response = await fetch(url, this.getFetchOptions());
            if (!response.ok) {
                if (this.handleAuthError(null, response)) {
                    return;
                }
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }

            const data = await response.json();
            this.processComponentsData(data);
            this.populateNamespaceSelect();
        } catch (error) {
            console.error('Failed to load components:', error);
            if (!this.handleAuthError(error, null)) {
                this.showError(`Failed to load components: ${error.message}`);
            }
        } finally {
            this.hideLoading();
        }
    }

    processComponentsData(data) {
        this.components = {};
        this.namespaceOrder = [];

        // Use ordered namespaces if available
        if (data.ordered_namespaces) {
            data.ordered_namespaces.forEach(nsData => {
                this.namespaceOrder.push(nsData.name);
                if (!this.components[nsData.name]) {
                    this.components[nsData.name] = {};
                }

                nsData.releases.forEach(release => {
                    if (!this.components[nsData.name][release.workload_name]) {
                        this.components[nsData.name][release.workload_name] = new Set();
                    }
                    this.components[nsData.name][release.workload_name].add(release.container_name);
                });
            });
        } else if (data.namespaces) {
            // Fallback for backward compatibility
            Object.entries(data.namespaces).forEach(([namespace, releases]) => {
                this.namespaceOrder.push(namespace);
                if (!this.components[namespace]) {
                    this.components[namespace] = {};
                }

                releases.forEach(release => {
                    if (!this.components[namespace][release.workload_name]) {
                        this.components[namespace][release.workload_name] = new Set();
                    }
                    this.components[namespace][release.workload_name].add(release.container_name);
                });
            });
        }
    }

    populateNamespaceSelect() {
        const select = document.getElementById('namespaceSelect');
        select.innerHTML = '<option value="">Select Namespace...</option>';

        // Use namespace order from configuration
        this.namespaceOrder.forEach(namespace => {
            if (this.components[namespace]) {
                const option = document.createElement('option');
                option.value = namespace;
                option.textContent = namespace;
                select.appendChild(option);
            }
        });
    }

    onNamespaceChange(namespace) {
        this.selectedComponent.namespace = namespace;
        this.selectedComponent.workload = '';
        this.selectedComponent.container = '';
        this.selectedComponent.workloadType = '';

        const workloadSelect = document.getElementById('workloadSelect');
        const containerSelect = document.getElementById('containerSelect');
        const loadBtn = document.getElementById('loadTimelineBtn');

        if (!namespace) {
            workloadSelect.disabled = true;
            containerSelect.disabled = true;
            loadBtn.disabled = true;
            workloadSelect.innerHTML = '<option value="">Select Workload...</option>';
            containerSelect.innerHTML = '<option value="">Select Container...</option>';
            return;
        }

        // Populate workloads
        workloadSelect.innerHTML = '<option value="">Select Workload...</option>';
        Object.keys(this.components[namespace] || {}).sort().forEach(workload => {
            const option = document.createElement('option');
            option.value = workload;
            option.textContent = workload;
            workloadSelect.appendChild(option);
        });

        workloadSelect.disabled = false;
        containerSelect.disabled = true;
        loadBtn.disabled = true;
        containerSelect.innerHTML = '<option value="">Select Container...</option>';
    }

    onWorkloadChange(workload) {
        this.selectedComponent.workload = workload;
        this.selectedComponent.container = '';
        this.selectedComponent.workloadType = '';

        const containerSelect = document.getElementById('containerSelect');
        const loadBtn = document.getElementById('loadTimelineBtn');

        if (!workload) {
            containerSelect.disabled = true;
            loadBtn.disabled = true;
            containerSelect.innerHTML = '<option value="">Select Container...</option>';
            return;
        }

        // Populate containers
        const namespace = this.selectedComponent.namespace;
        const containers = Array.from(this.components[namespace][workload] || []);

        containerSelect.innerHTML = '<option value="">Select Container...</option>';
        containers.sort().forEach(container => {
            const option = document.createElement('option');
            option.value = container;
            option.textContent = container;
            containerSelect.appendChild(option);
        });

        containerSelect.disabled = false;
        loadBtn.disabled = true;
    }

    onContainerChange(container) {
        this.selectedComponent.container = container;
        const loadBtn = document.getElementById('loadTimelineBtn');
        loadBtn.disabled = !container;
    }

    async loadTimeline() {
        const { namespace, workload, container } = this.selectedComponent;

        if (!namespace || !workload || !container) {
            this.showError('Please select namespace, workload, and container');
            return;
        }

        this.showLoading();
        this.hideError();

        try {
            const response = await fetch(`${this.basePath}/api/releases/history/${encodeURIComponent(this.selectedClient)}/${encodeURIComponent(this.selectedEnvironment)}/${encodeURIComponent(namespace)}/${encodeURIComponent(workload)}/${encodeURIComponent(container)}`, this.getFetchOptions());
            if (!response.ok) {
                if (this.handleAuthError(null, response)) {
                    return;
                }
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }

            const data = await response.json();
            this.releases = data.history.releases || [];

            // Extract workload type from the first release if available
            if (this.releases.length > 0 && this.releases[0].workload_type) {
                this.selectedComponent.workloadType = this.releases[0].workload_type;
            }

            this.updateComponentInfo();
            this.renderTimeline();
            await this.renderEnvironmentComparison();
            this.showTimelineContent();
        } catch (error) {
            console.error('Failed to load timeline:', error);
            if (!this.handleAuthError(error, null)) {
                this.showError(`Failed to load timeline: ${error.message}`);
            }
        } finally {
            this.hideLoading();
        }
    }

    updateComponentInfo() {
        const { namespace, workload, container } = this.selectedComponent;

        document.getElementById('currentNamespace').textContent = namespace;
        document.getElementById('currentWorkload').textContent = workload;
        document.getElementById('currentContainer').textContent = container;

        // Update the total releases to show environment count instead
        const environments = this.clientsEnvironments?.clients_environments[this.selectedClient] || [];
        document.getElementById('totalReleases').textContent = `${environments.length} environments`;

        // Show badge generation button when component info is displayed
        const badgeBtn = document.getElementById('generateBadgeBtn');
        if (badgeBtn) {
            badgeBtn.style.display = 'flex';
        }
    }

    renderTimeline() {
        const chart = document.getElementById('timelineChart');

        if (this.releases.length === 0) {
            chart.innerHTML = '<div class="empty-timeline">No release history found</div>';
            return;
        }

        const timelineHTML = `
            <div class="timeline-axis">
                <div class="timeline-line"></div>
                ${this.releases.map((release, index) => {
                    const changeType = this.getChangeType(release, index);
                    return `
                    <div class="timeline-event ${index === 0 ? 'latest' : ''} ${changeType}">
                        <div class="timeline-event-content">
                            <div class="timeline-event-header">
                                <span class="timeline-event-tag">${this.escapeHtml(release.image_tag)}</span>
                                <span class="change-indicator ${changeType}">${this.getChangeIndicator(changeType)}</span>
                                <span class="timeline-event-time">${this.formatTimestamp(release.last_seen)}</span>
                            </div>
                            <div class="timeline-event-details">
                                <div class="timeline-event-detail">
                                    <span class="label">Image:</span>
                                    <span class="value">${this.escapeHtml(release.image_name+":"+release.image_tag)}</span>
                                </div>
                                <div class="timeline-event-detail">
                                    <span class="label">Repository:</span>
                                    <span class="value">${this.escapeHtml(release.image_repo || 'N/A')}</span>
                                </div>
                                <div class="timeline-event-detail">
                                    <span class="label">Image SHA:</span>
                                    <span class="value image-sha-timeline" title="${this.escapeHtml(release.image_sha || '')}">${this.formatImageSHA(release.image_sha)}</span>
                                </div>
                                <div class="timeline-event-detail">
                                    <span class="label">First Seen:</span>
                                    <span class="value">${this.formatTimestamp(release.first_seen)}</span>
                                </div>
                            </div>
                        </div>
                    </div>
                    `;
                }).join('')}
            </div>
        `;

        chart.innerHTML = timelineHTML;
    }

    async renderEnvironmentComparison() {
        const grid = document.getElementById('releasesGrid');
        const { namespace, workload, container } = this.selectedComponent;

        if (!this.clientsEnvironments || !this.selectedClient) {
            grid.innerHTML = '<div class="empty-releases">Unable to load environment data</div>';
            return;
        }

        const environments = this.clientsEnvironments.clients_environments[this.selectedClient] || [];

        if (environments.length === 0) {
            grid.innerHTML = '<div class="empty-releases">No environments found for this client</div>';
            return;
        }

        // Fetch current releases for each environment
        const environmentReleases = await this.fetchEnvironmentReleases(environments, namespace, workload, container);

        if (Object.keys(environmentReleases).length === 0) {
            grid.innerHTML = '<div class="empty-releases">Component not found in any environment</div>';
            return;
        }

        // Detect differences across environments
        const differences = this.detectEnvironmentDifferences(environmentReleases);

        // Update the header to reflect cross-environment comparison
        const releasesList = document.getElementById('releasesList');
        const header = releasesList.querySelector('h4');
        if (header) {
            header.textContent = 'Cross-Environment Comparison';
        }

        // Render environment comparison cards
        grid.innerHTML = environments.map(envName => {
            const release = environmentReleases[envName];
            const hasDifferences = differences.environments.includes(envName);

            if (!release) {
                return `
                <div class="release-card environment-missing">
                    <div class="release-card-header">
                        <span class="release-card-tag environment-name">${this.escapeHtml(envName)}</span>
                        <span class="change-indicator new-deployment">❌ Not Found</span>
                    </div>
                    <div class="release-card-details">
                        <div class="release-card-detail">
                            <span class="label">Status:</span>
                            <span class="value">Component not deployed in this environment</span>
                        </div>
                    </div>
                </div>
                `;
            }

            const changeType = hasDifferences ? 'image-change' : 'latest';
            const changeIndicator = hasDifferences ? '⚠️ Different' : '✅ Consistent';

            return `
            <div class="release-card environment-card ${changeType}">
                <div class="release-card-header">
                    <span class="release-card-tag environment-name">${this.escapeHtml(envName)}</span>
                    <span class="change-indicator ${changeType}">${changeIndicator}</span>
                    <span class="release-card-time">${this.formatTimestamp(release.last_seen)}</span>
                </div>
                <div class="release-card-details">
                    <div class="release-card-detail">
                        <span class="label">Image Tag:</span>
                        <span class="value ${differences.tags.includes(envName) ? 'difference-highlight' : ''}">${this.escapeHtml(release.image_tag)}</span>
                    </div>
                    <div class="release-card-detail">
                        <span class="label">Image SHA:</span>
                        <span class="value image-sha-timeline ${differences.shas.includes(envName) ? 'difference-highlight' : ''}" title="${this.escapeHtml(release.image_sha || '')}">${this.formatImageSHA(release.image_sha)}</span>
                    </div>
                    <div class="release-card-detail">
                        <span class="label">Image:</span>
                        <span class="value">${this.escapeHtml(release.image_name+":"+release.image_tag)}</span>
                    </div>
                    <div class="release-card-detail">
                        <span class="label">Repository:</span>
                        <span class="value">${this.escapeHtml(release.image_repo || 'N/A')}</span>
                    </div>
                    <div class="release-card-detail">
                        <span class="label">Last Seen:</span>
                        <span class="value">${this.formatTimestamp(release.last_seen)}</span>
                    </div>
                </div>
            </div>
            `;
        }).join('');
    }

    async fetchEnvironmentReleases(environments, namespace, workload, container) {
        const environmentReleases = {};

        // Fetch release history for each environment in parallel using the more efficient endpoint
        const fetchPromises = environments.map(async (envName) => {
            try {
                const url = `${this.basePath}/api/releases/history/${encodeURIComponent(this.selectedClient)}/${encodeURIComponent(envName)}/${encodeURIComponent(namespace)}/${encodeURIComponent(workload)}/${encodeURIComponent(container)}`;
                const response = await fetch(url, this.getFetchOptions());

                if (!response.ok) {
                    console.warn(`Failed to fetch releases for environment ${envName}:`, response.status);
                    return;
                }

                const data = await response.json();

                // Extract the most recent release from the history endpoint response
                const componentRelease = this.extractCurrentReleaseFromHistory(data);
                if (componentRelease) {
                    environmentReleases[envName] = componentRelease;
                }
            } catch (error) {
                console.warn(`Error fetching releases for environment ${envName}:`, error);
            }
        });

        await Promise.all(fetchPromises);
        return environmentReleases;
    }

    extractCurrentReleaseFromHistory(historyData) {
        // Extract the most recent release from the history endpoint response
        // History endpoint returns: {component: {...}, history: {releases: [...], total: N}, timestamp: "..."}
        let releases = [];

        if (historyData.history && historyData.history.releases) {
            releases = historyData.history.releases;
        } else if (historyData.releases) {
            // Fallback for direct releases array (in case API structure changes)
            releases = historyData.releases;
        }

        // Return the most recent release (first in the array, as they're sorted by last_seen desc)
        if (releases && releases.length > 0) {
            return releases[0];
        }

        return null;
    }

    detectEnvironmentDifferences(environmentReleases) {
        const environments = Object.keys(environmentReleases);
        const tags = new Set();
        const shas = new Set();

        // Collect all unique tags and SHAs
        environments.forEach(env => {
            const release = environmentReleases[env];
            tags.add(release.image_tag);
            if (release.image_sha) {
                shas.add(release.image_sha);
            }
        });

        // Determine which environments have differences
        const differentTags = [];
        const differentShas = [];
        const differentEnvironments = [];

        if (tags.size > 1 || shas.size > 1) {
            // Find the most common tag and SHA to use as baseline
            const tagCounts = {};
            const shaCounts = {};

            environments.forEach(env => {
                const release = environmentReleases[env];
                tagCounts[release.image_tag] = (tagCounts[release.image_tag] || 0) + 1;
                if (release.image_sha) {
                    shaCounts[release.image_sha] = (shaCounts[release.image_sha] || 0) + 1;
                }
            });

            const mostCommonTag = Object.keys(tagCounts).reduce((a, b) => tagCounts[a] > tagCounts[b] ? a : b);
            const mostCommonSha = Object.keys(shaCounts).length > 0 ?
                Object.keys(shaCounts).reduce((a, b) => shaCounts[a] > shaCounts[b] ? a : b) : null;

            environments.forEach(env => {
                const release = environmentReleases[env];
                let hasDifference = false;

                if (release.image_tag !== mostCommonTag) {
                    differentTags.push(env);
                    hasDifference = true;
                }

                if (mostCommonSha && release.image_sha && release.image_sha !== mostCommonSha) {
                    differentShas.push(env);
                    hasDifference = true;
                }

                if (hasDifference) {
                    differentEnvironments.push(env);
                }
            });
        }

        return {
            environments: differentEnvironments,
            tags: differentTags,
            shas: differentShas
        };
    }

    showTimelineContent() {
        document.getElementById('emptyState').style.display = 'none';
        document.getElementById('componentInfo').style.display = 'block';
        document.getElementById('timelineContainer').style.display = 'block';
        document.getElementById('releasesList').style.display = 'block';
    }

    showLoading() {
        document.getElementById('loading').style.display = 'flex';
    }

    hideLoading() {
        document.getElementById('loading').style.display = 'none';
    }

    showError(message) {
        document.getElementById('errorMessage').textContent = message;
        document.getElementById('error').style.display = 'flex';
    }

    hideError() {
        document.getElementById('error').style.display = 'none';
    }

    formatTimestamp(timestamp) {
        if (!timestamp) return '-';
        const date = new Date(timestamp);
        return date.toLocaleString(undefined, {
            year: 'numeric',
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit',
            hour12: false
        });
    }

    formatImageSHA(sha) {
        if (!sha) return '-';
        // Show first 12 characters of SHA256 for readability
        return sha.length > 12 ? sha.substring(0, 12) : sha;
    }

    getChangeType(release, index) {
        if (index === 0) return 'latest';

        const previousRelease = this.releases[index - 1];
        if (!previousRelease) return 'new-deployment';

        // If image SHA changed, it's a real deployment change
        if (release.image_sha && previousRelease.image_sha && release.image_sha !== previousRelease.image_sha) {
            return 'image-change';
        }

        // If only tag changed but SHA is the same, it's just a tag update
        if (release.image_tag !== previousRelease.image_tag &&
            release.image_sha && previousRelease.image_sha &&
            release.image_sha === previousRelease.image_sha) {
            return 'tag-update';
        }

        // Default case - treat as deployment change
        return 'image-change';
    }

    getChangeIndicator(changeType) {
        switch (changeType) {
            case 'image-change':
                return '🔄 Deployment';
            case 'tag-update':
                return '🏷️ Tag Update';
            case 'new-deployment':
                return '🆕 New';
            case 'latest':
            default:
                return '';
        }
    }

    escapeHtml(text) {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    generateBadge() {
        const { namespace, workload, container, workloadType } = this.selectedComponent;

        if (!namespace || !workload || !container) {
            this.showError('No component selected. Please load a timeline first.');
            return;
        }

        if (!this.selectedClient || !this.selectedEnvironment) {
            this.showError('Client and environment information not available.');
            return;
        }

        if (!this.apiKey) {
            this.showError('API key is required for badge generation. Please add ?apikey=YOUR_API_KEY to the URL.');
            return;
        }

        // Use workload type if available, otherwise try to infer from workload name
        let workloadKind = workloadType;
        if (!workloadKind) {
            // Fallback: try to infer from common naming patterns
            const workloadLower = workload.toLowerCase();
            if (workloadLower.includes('deployment') || workloadLower.includes('deploy')) {
                workloadKind = 'Deployment';
            } else if (workloadLower.includes('statefulset') || workloadLower.includes('sts')) {
                workloadKind = 'StatefulSet';
            } else if (workloadLower.includes('daemonset') || workloadLower.includes('ds')) {
                workloadKind = 'DaemonSet';
            } else {
                // Default to Deployment if we can't determine
                workloadKind = 'Deployment';
            }
        }

        // Build the badges.html URL with pre-filled parameters
        const badgeParams = new URLSearchParams({
            'apikey': this.apiKey,
            'client': this.selectedClient,
            'env': this.selectedEnvironment,
            'workload-kind': workloadKind,
            'workload-name': workload,
            'container': container
        });

        const badgeUrl = `${this.basePath}/badges.html?${badgeParams.toString()}`;

        // Navigate to the badges page
        window.location.href = badgeUrl;
    }
}

// Initialize timeline when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    new Timeline();
});
