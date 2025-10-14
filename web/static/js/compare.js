class EnvironmentComparison {
    constructor() {
        this.apiKey = '';
        this.selectedClient = '';
        this.basePath = '';
        this.environments = [];
        this.environmentData = {};
        this.components = new Map(); // Map of component keys to environment data
    }

    async init() {
        this.apiKey = this.extractAPIKey();
        this.basePath = this.getBasePath();
        this.parseUrlParams();
        this.bindEvents();
        this.updateNavigationLinks();

        if (!this.selectedClient) {
            this.showError('No client specified. Please select a client from the dashboard.');
            return;
        }

        await this.loadComparison();
    }

    // Extract API key from URL query parameters
    extractAPIKey() {
        const urlParams = new URLSearchParams(window.location.search);
        return urlParams.get('apikey') || '';
    }

    // Get base path from current URL
    getBasePath() {
        const path = window.location.pathname;
        const segments = path.split('/').filter(s => s);

        // If we're at a specific page, remove the filename
        if (segments.length > 0 && segments[segments.length - 1].includes('.html')) {
            segments.pop();
        }

        return segments.length > 0 ? '/' + segments.join('/') : '';
    }

    // Parse URL parameters
    parseUrlParams() {
        const urlParams = new URLSearchParams(window.location.search);
        this.selectedClient = urlParams.get('client') || '';

        // Update client name display
        document.getElementById('clientName').textContent = "Selected Client: " + this.selectedClient.toUpperCase() || 'Unknown Client';
    }

    // Bind event handlers
    bindEvents() {

        document.getElementById('backToHome').addEventListener('click', () => {
            const homeUrl = this.apiKey
                ? `${this.basePath}/index.html?apikey=${encodeURIComponent(this.apiKey)}`
                : `${this.basePath}/index.html`;
            window.location.href = homeUrl;
        });

        // Search functionality
        const searchInput = document.getElementById('searchInput');
        const clearSearch = document.getElementById('clearSearch');

        if (searchInput) {
            searchInput.addEventListener('input', () => {
                this.filterTable(searchInput.value);
            });
        }

        if (clearSearch) {
            clearSearch.addEventListener('click', () => {
                searchInput.value = '';
                this.filterTable('');
            });
        }
    }

    // Update navigation links to preserve API key
    updateNavigationLinks() {
        if (this.apiKey) {
            const backBtn = document.getElementById('backToDashboard');
            // Already handled in bindEvents
        }
    }

    // Create fetch options with API key authentication
    getFetchOptions(options = {}) {
        const fetchOptions = {
            ...options,
            headers: {
                ...options.headers
            }
        };

        if (this.apiKey) {
            fetchOptions.headers['Authorization'] = `Bearer ${this.apiKey}`;
        }

        return fetchOptions;
    }

    // Show loading state
    showLoading() {
        document.getElementById('loading').style.display = 'block';
        document.getElementById('comparisonContainer').style.display = 'none';
        document.getElementById('emptyState').style.display = 'none';
        document.getElementById('error').style.display = 'none';
    }

    // Hide loading state
    hideLoading() {
        document.getElementById('loading').style.display = 'none';
    }

    // Show error message
    showError(message) {
        document.getElementById('error').style.display = 'block';
        document.getElementById('errorMessage').textContent = message;
        document.getElementById('comparisonContainer').style.display = 'none';
        document.getElementById('emptyState').style.display = 'none';
        this.hideLoading();
    }

    // Show empty state
    showEmptyState() {
        document.getElementById('emptyState').style.display = 'block';
        document.getElementById('comparisonContainer').style.display = 'none';
        document.getElementById('error').style.display = 'none';
        this.hideLoading();
    }

    // Show comparison results
    showComparison() {
        document.getElementById('comparisonContainer').style.display = 'block';
        document.getElementById('emptyState').style.display = 'none';
        document.getElementById('error').style.display = 'none';
        this.hideLoading();
    }

    // Handle authentication errors
    handleAuthError(error, response) {
        if (response && response.status === 401) {
            this.showError('Authentication failed. Please check your API key.');
            return true;
        }
        if (error && error.message.includes('401')) {
            this.showError('Authentication failed. Please check your API key.');
            return true;
        }
        return false;
    }

    // Load environments and comparison data
    async loadComparison() {
        this.showLoading();

        try {
            // First, get the list of environments for this client
            await this.loadEnvironments();

            if (this.environments.length === 0) {
                this.showEmptyState();
                return;
            }

            // Then fetch current releases for each environment
            await this.fetchEnvironmentData();

            // Process and render the comparison
            this.processComparisonData();
            this.renderComparison();
            this.showComparison();

        } catch (error) {
            console.error('Failed to load comparison:', error);
            if (!this.handleAuthError(error, null)) {
                this.showError(`Failed to load comparison: ${error.message}`);
            }
        }
    }

    // Load environments for the selected client
    async loadEnvironments() {
        const response = await fetch(`${this.basePath}/api/clients-environments`, this.getFetchOptions());

        if (!response.ok) {
            if (this.handleAuthError(null, response)) {
                return;
            }
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }

        const data = await response.json();
        this.environments = data.clients_environments[this.selectedClient] || [];
    }

    // Fetch current releases for all environments
    async fetchEnvironmentData() {
        const fetchPromises = this.environments.map(async (envName) => {
            try {
                const params = new URLSearchParams();
                params.append('client_name', this.selectedClient);
                params.append('env_name', envName);

                const url = `${this.basePath}/api/releases/current?${params}`;
                const response = await fetch(url, this.getFetchOptions());

                if (!response.ok) {
                    console.warn(`Failed to fetch releases for environment ${envName}:`, response.status);
                    return;
                }

                const data = await response.json();
                this.environmentData[envName] = data;
            } catch (error) {
                console.warn(`Error fetching releases for environment ${envName}:`, error);
            }
        });

        await Promise.all(fetchPromises);
    }

    // Process comparison data to identify components and differences
    processComparisonData() {
        this.components.clear();

        // Collect all unique components across all environments
        this.environments.forEach(envName => {
            const envData = this.environmentData[envName];
            if (!envData || !envData.namespaces) return;

            Object.entries(envData.namespaces).forEach(([namespace, releases]) => {
                releases.forEach(release => {
                    const componentKey = `${namespace}/${release.workload_name}/${release.container_name}`;

                    if (!this.components.has(componentKey)) {
                        this.components.set(componentKey, {
                            namespace,
                            workload_name: release.workload_name,
                            container_name: release.container_name,
                            environments: {}
                        });
                    }

                    this.components.get(componentKey).environments[envName] = {
                        image_tag: release.image_tag,
                        image_sha: release.image_sha,
                        last_seen: release.last_seen
                    };
                });
            });
        });
    }

    // Render the comparison table
    renderComparison() {
        this.renderTableHeader();
        this.renderTableBody();
        this.updateStats();
    }

    // Render table header with environment columns
    renderTableHeader() {
        const thead = document.querySelector('#comparisonTable thead tr');

        // Clear existing environment headers (keep component header)
        const existingHeaders = thead.querySelectorAll('.environment-header');
        existingHeaders.forEach(header => header.remove());

        // Add environment headers
        this.environments.forEach(envName => {
            const th = document.createElement('th');
            th.className = 'environment-header';
            th.textContent = envName;
            thead.appendChild(th);
        });
    }

    // Render table body with component comparison data
    renderTableBody() {
        const tbody = document.getElementById('comparisonTableBody');
        tbody.innerHTML = '';

        if (this.components.size === 0) {
            const row = document.createElement('tr');
            row.innerHTML = `
                <td colspan="${this.environments.length + 1}" class="missing-component">
                    No components found across any environments
                </td>
            `;
            tbody.appendChild(row);
            return;
        }

        // Sort components by namespace, then workload, then container
        const sortedComponents = Array.from(this.components.entries()).sort((a, b) => {
            const [keyA, compA] = a;
            const [keyB, compB] = b;

            if (compA.namespace !== compB.namespace) {
                return compA.namespace.localeCompare(compB.namespace);
            }
            if (compA.workload_name !== compB.workload_name) {
                return compA.workload_name.localeCompare(compB.workload_name);
            }
            return compA.container_name.localeCompare(compB.container_name);
        });

        sortedComponents.forEach(([componentKey, component]) => {
            const row = document.createElement('tr');

            // Component info cell
            const componentCell = document.createElement('td');
            componentCell.className = 'component-cell';
            componentCell.innerHTML = `
                <div class="component-info">
                    <div class="component-namespace" title="Namespace">${this.escapeHtml(component.namespace)}</div>
                    <div class="component-workload" title="Workload">${this.escapeHtml(component.workload_name)}</div>
                    <div class="component-container" title="Container">${this.escapeHtml(component.container_name)}</div>
                </div>
            `;
            row.appendChild(componentCell);

            // Detect differences for this component
            const differences = this.detectComponentDifferences(component);

            // Environment cells
            this.environments.forEach(envName => {
                const envCell = document.createElement('td');
                envCell.className = 'environment-cell';

                const envData = component.environments[envName];
                if (envData) {
                    const isDifferent = differences.differentEnvironments.includes(envName);
                    const releaseClass = isDifferent ? 'difference-highlight' : 'consistent-component';

                    envCell.innerHTML = `
                        <div class="release-info ${releaseClass}">
                            <div class="release-tag">${this.escapeHtml(envData.image_tag)}</div>
                            <div class="release-sha" title="${this.escapeHtml(envData.image_sha)}">${this.formatImageSHA(envData.image_sha)}</div>
                        </div>
                    `;
                } else {
                    envCell.innerHTML = `
                        <div class="missing-component">
                            <div>Not Found</div>
                        </div>
                    `;
                }

                row.appendChild(envCell);
            });

            tbody.appendChild(row);
        });
    }

    // Detect differences for a specific component across environments
    detectComponentDifferences(component) {
        const envNames = Object.keys(component.environments);
        if (envNames.length <= 1) {
            return { hasDifferences: false, differentEnvironments: [] };
        }

        // Get all unique tag/sha combinations
        const tagCombinations = new Map();
        const shaCombinations = new Map();

        envNames.forEach(envName => {
            const envData = component.environments[envName];
            const tag = envData.image_tag;
            const sha = envData.image_sha;

            if (!tagCombinations.has(tag)) {
                tagCombinations.set(tag, []);
            }
            tagCombinations.get(tag).push(envName);

            if (!shaCombinations.has(sha)) {
                shaCombinations.set(sha, []);
            }
            shaCombinations.get(sha).push(envName);
        });

        // Check if there are differences
        const hasDifferentTags = tagCombinations.size > 1;
        const hasDifferentShas = shaCombinations.size > 1;
        const hasDifferences = hasDifferentTags || hasDifferentShas;

        if (!hasDifferences) {
            return { hasDifferences: false, differentEnvironments: [] };
        }

        // Find the baseline (most common combination)
        let baselineTag = '';
        let baselineSha = '';
        let maxTagCount = 0;
        let maxShaCount = 0;

        tagCombinations.forEach((envs, tag) => {
            if (envs.length > maxTagCount) {
                maxTagCount = envs.length;
                baselineTag = tag;
            }
        });

        shaCombinations.forEach((envs, sha) => {
            if (envs.length > maxShaCount) {
                maxShaCount = envs.length;
                baselineSha = sha;
            }
        });

        // Find environments that differ from baseline
        const differentEnvironments = envNames.filter(envName => {
            const envData = component.environments[envName];
            return envData.image_tag !== baselineTag || envData.image_sha !== baselineSha;
        });

        return { hasDifferences: true, differentEnvironments };
    }

    // Update statistics
    updateStats() {
        const totalEnvironments = this.environments.length;
        const totalComponents = this.components.size;

        let totalDifferences = 0;
        this.components.forEach(component => {
            const differences = this.detectComponentDifferences(component);
            if (differences.hasDifferences) {
                totalDifferences++;
            }
        });

        document.getElementById('totalEnvironments').textContent = totalEnvironments;
        document.getElementById('totalComponents').textContent = totalComponents;
        document.getElementById('totalDifferences').textContent = totalDifferences;
    }

    // Format image SHA for display
    formatImageSHA(sha) {
        if (!sha) return 'N/A';
        return sha.length > 12 ? sha.substring(0, 12) + '...' : sha;
    }

    // Filter table based on search term
    filterTable(searchTerm) {
        const tbody = document.getElementById('comparisonTableBody');
        if (!tbody) return;

        const rows = tbody.querySelectorAll('tr');
        const term = searchTerm.toLowerCase().trim();

        let visibleCount = 0;

        rows.forEach(row => {
            if (term === '') {
                row.style.display = '';
                visibleCount++;
                return;
            }

            // Get component information
            const componentCell = row.querySelector('.component-cell');
            if (!componentCell) {
                row.style.display = 'none';
                return;
            }

            const namespace = componentCell.querySelector('.component-namespace')?.textContent || '';
            const workload = componentCell.querySelector('.component-workload')?.textContent || '';
            const container = componentCell.querySelector('.component-container')?.textContent || '';

            // Get environment data (image tags)
            const environmentCells = row.querySelectorAll('.environment-cell');
            let environmentData = '';
            environmentCells.forEach(cell => {
                const tagElement = cell.querySelector('.release-tag');
                if (tagElement) {
                    environmentData += ' ' + tagElement.textContent;
                }
            });

            // Check if search term matches any field
            const searchableText = `${namespace} ${workload} ${container} ${environmentData}`.toLowerCase();

            if (searchableText.includes(term)) {
                row.style.display = '';
                visibleCount++;
            } else {
                row.style.display = 'none';
            }
        });

        // Update component count to show filtered results
        const totalComponentsElement = document.getElementById('totalComponents');
        if (totalComponentsElement) {
            const totalComponents = this.components.size;
            if (term === '') {
                totalComponentsElement.textContent = totalComponents;
            } else {
                totalComponentsElement.textContent = `${visibleCount} / ${totalComponents}`;
            }
        }
    }

    // Escape HTML to prevent XSS
    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// Initialize the comparison when the page loads
document.addEventListener('DOMContentLoaded', () => {
    const comparison = new EnvironmentComparison();
    comparison.init();
});
