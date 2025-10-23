class Dashboard {
    constructor() {
        this.releases = [];
        this.filteredReleases = [];
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

    updateCompareButtonVisibility() {
        const compareBtn = document.querySelectorAll('.compare-btn');
        compareBtn.forEach(btn => {
            // if parent div has class "selected", show the button
            btn.style.display = btn.parentElement.parentElement.parentElement.classList.contains('selected') ? 'inline-block' : 'none';
            // else, hide the button
        });
    }

    openCompare() {
        if (this.selectedClient) {
            const compareUrl = this.apiKey
                ? `${this.basePath}/compare.html?client=${encodeURIComponent(this.selectedClient)}&apikey=${encodeURIComponent(this.apiKey)}`
                : `${this.basePath}/compare.html?client=${encodeURIComponent(this.selectedClient)}`;
            window.location.href = compareUrl;
        }
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

    init() {
        this.bindEvents();
        this.updateNavigationLinks();
        this.loadConfig();
        this.loadSlaveFilteredReleases();
    }

    // Show or hide the header in master mode
    showHeader(show) {
        const header = document.querySelector('header');
        if (header) {
            header.style.display = show ? '' : 'none';
        }
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
            const timelineLink = document.querySelector('a[href="timeline.html"]');
            const badgeLink = document.querySelector('a[href="badges.html"]');
            const dashboardLink = document.querySelector('a[href="index.html"]');

            if (timelineLink) {
                timelineLink.href = `${this.basePath}/timeline.html?apikey=${encodeURIComponent(this.apiKey)}`;
            }
            if (badgeLink) {
                badgeLink.href = `${this.basePath}/badges.html?apikey=${encodeURIComponent(this.apiKey)}`;
            }
            if (dashboardLink) {
                dashboardLink.href = `${this.basePath}/index.html?apikey=${encodeURIComponent(this.apiKey)}`;
            }
        }
    }

    async loadConfig() {
        try {
            const response = await fetch(`${this.basePath}/api/config`, this.getFetchOptions());
            if (response.ok) {
                const config = await response.json();
                this.config = config;

                // Determine UI mode based on application mode and API key type
                const isAdminKey = config.api_key_type?.is_admin || false;
                const authenticatedClient = config.api_key_type?.authenticated_client || '';

                if (config.mode === 'master' && isAdminKey) {
                    // Admin API key in master mode - show full control panel
                    const collectBtn = document.getElementById('collectBtn');
                    if (collectBtn) {
                        collectBtn.style.display = 'none';
                    }
                    this.showHeader(false);
                    await this.loadControlPanel();
                } else if (config.mode === 'master' && !isAdminKey && authenticatedClient) {
                    // Standard API key in master mode - show limited control panel for specific client
                    const collectBtn = document.getElementById('collectBtn');
                    if (collectBtn) {
                        collectBtn.style.display = 'none';
                    }
                    this.showHeader(false);
                    this.selectedClient = authenticatedClient;
                    await this.loadControlPanel();
                } else {
                    // Slave mode - traditional behavior
                    this.showHeader(true);
                    this.hideControlPanel();
                    this.selectedClient = config.client_name;
                    this.selectedEnvironment = config.env_name;
                }
            }
        } catch (error) {
            console.warn('Failed to load config:', error);
        }
    }

    async loadControlPanel() {
        try {
            const response = await fetch(`${this.basePath}/api/clients-environments`, this.getFetchOptions());
            if (response.ok) {
                const data = await response.json();
                this.clientsEnvironments = data;
                this.renderControlPanel(data);
                this.updateCompareButtonVisibility();

                // Set up periodic refresh for ping statuses
                this.startPingStatusRefresh();

                // Try to restore saved selection after control panel is loaded
                this.restoreSelectionFromStorage();
            }
        } catch (error) {
            console.error('Failed to load control panel data:', error);
        }
    }

    restoreSelectionFromStorage() {
        const savedSelection = this.loadSelectionFromStorage();
        if (savedSelection && this.clientsEnvironments) {
            const { client, environment } = savedSelection;

            // Check if the saved client and environment still exist
            const clientExists = this.clientsEnvironments.clients_environments[client];
            const environmentExists = clientExists && clientExists.includes(environment);

            if (clientExists && environmentExists) {
                // Restore the selection
                this.selectedClient = client;
                this.selectedEnvironment = environment;
                this.updateClientSelection();
                this.loadMasterFilteredReleases();
            } else {
                // Clear invalid stored selection
                this.clearStoredSelection();
            }
        }
    }

    startPingStatusRefresh() {
        // Refresh ping statuses every 30 seconds
        if (this.pingRefreshInterval) {
            clearInterval(this.pingRefreshInterval);
        }

        this.pingRefreshInterval = setInterval(async () => {
            try {
                const response = await fetch(`${this.basePath}/api/clients-environments`, this.getFetchOptions());
                if (response.ok) {
                    const data = await response.json();
                    this.clientsEnvironments = data;
                    this.updatePingStatuses();
                }
            } catch (error) {
                console.error('Failed to refresh ping statuses:', error);
            }
        }, 30000);
    }

    updatePingStatuses() {
        if (!this.clientsEnvironments?.ping_statuses) return;

        const pingStatuses = this.clientsEnvironments.ping_statuses;

        // Update client status indicators
        Object.entries(pingStatuses).forEach(([clientName, envStatuses]) => {
            const clientRow = document.querySelector(`[data-client="${clientName}"]`);
            if (!clientRow) return;

            // Update client overall status
            let clientStatus = 'never';
            let latestPing = null;

            Object.values(envStatuses).forEach(envData => {
                const envStatus = envData.status;
                const envLastPing = envData.last_ping;

                if (envStatus === 'offline' || clientStatus === 'never') {
                    clientStatus = envStatus;
                } else if (envStatus === 'warning' && clientStatus === 'online') {
                    clientStatus = envStatus;
                } else if (envStatus === 'online' && clientStatus === 'never') {
                    clientStatus = envStatus;
                }

                // Track the latest ping time
                if (envLastPing && (!latestPing || new Date(envLastPing) > new Date(latestPing))) {
                    latestPing = envLastPing;
                }
            });

            // Update status indicator
            const clientIndicator = clientRow.querySelector('.status-indicator');
            if (clientIndicator) {
                clientIndicator.className = `status-indicator ${clientStatus}`;
            }

            // Update status text
            const statusText = clientRow.querySelector('.status-text');
            if (statusText) {
                statusText.className = `status-text ${clientStatus}`;
                statusText.textContent = this.getStatusText(clientStatus);
            }

            // Update last ping time
            const pingTime = clientRow.querySelector('.ping-time');
            if (pingTime) {
                pingTime.textContent = latestPing ? this.formatTimestamp(latestPing) : 'Never';
            }

            // Update environment status dots
            Object.entries(envStatuses).forEach(([envName, envData]) => {
                const envTag = clientRow.querySelector(`[data-env="${envName}"]`);
                if (envTag) {
                    const envDot = envTag.querySelector('.env-status-dot');
                    if (envDot) {
                        envDot.className = `env-status-dot ${envData.status}`;
                    }
                }
            });
        });
    }

    getSelectorTitle() {
        const isAdminKey = this.config?.api_key_type?.is_admin || false;
        const authenticatedClient = this.config?.api_key_type?.authenticated_client || '';

        if (!isAdminKey && authenticatedClient) {
            return `Select Environment for ${authenticatedClient}`;
        }
        return 'Select Client & Environment';
    }

    renderStepIndicator() {
        const isAdminKey = this.config?.api_key_type?.is_admin || false;
        const authenticatedClient = this.config?.api_key_type?.authenticated_client || '';

        if (!isAdminKey && authenticatedClient) {
            // For standard API keys, only show environment selection step
            return `
                <div class="step-indicator">
                    <div class="step" id="step2">
                        <div class="step-number">1</div>
                        <span>Choose Environment</span>
                    </div>
                </div>
            `;
        }

        // For admin API keys, show both steps
        return `
            <div class="step-indicator">
                <div class="step" id="step1">
                    <div class="step-number">1</div>
                    <span>Choose Client</span>
                </div>
                <div class="step-arrow">→</div>
                <div class="step" id="step2">
                    <div class="step-number">2</div>
                    <span>Choose Environment</span>
                </div>
            </div>
        `;
    }

    renderControlPanel(data) {
        const container = document.querySelector('.container');
        if (!container) return;

        // Create control panel HTML
        const controlPanelHTML = `
            <div class="control-panel" id="controlPanel">

                <div class="no-selection-state" id="noSelectionState">
                <div class="no-selection-icon">🎯</div>
                <p class="no-selection-subtitle">
                Choose a client and environment combination above to view release data.<br>
                Each client represents a different deployment target with its own environments.
                </p>
                </div>

                <div class="control-panel-stats">
                    <div class="stat-card">
                        <span class="stat-number">${data.statistics.total_clients}</span>
                        <span class="stat-label">Clients</span>
                    </div>
                    <div class="stat-card">
                        <span class="stat-number">${data.statistics.total_environments}</span>
                        <span class="stat-label">Environments</span>
                    </div>
                    <div class="stat-card">
                        <span class="stat-number">${data.statistics.total_releases}</span>
                        <span class="stat-label">Total Releases</span>
                    </div>
                </div>

                <div class="client-env-selector">
                    <div class="selector-header">
                        <h3 class="selector-title">${this.getSelectorTitle()}</h3>
                        <button class="clear-selection-btn" onclick="dashboard.clearSelection()">Clear Selection</button>
                    </div>

                    ${this.renderStepIndicator()}

                    <div class="clients-table-container" id="clientsTableContainer">
                        <table class="clients-table" id="clientsTable">
                            <thead>
                                <tr>
                                    <th class="client-name-header">Client</th>
                                    <th class="env-count-header">Environments</th>
                                    <th class="status-header">Status</th>
                                    <th class="last-ping-header">Last Ping</th>
                                    <th class="environments-header">Environments</th>
                                </tr>
                            </thead>
                            <tbody id="clientsTableBody">
                                ${this.renderClientsTable(data.clients_environments)}
                            </tbody>
                        </table>
                    </div>

                    <div class="current-selection" id="currentSelection" style="display: none;">
                        <p class="selection-text">
                            Viewing: <span class="selection-highlight" id="selectionText"></span>
                        </p>
                    </div>
                </div>


            </div>
        `;

        // Insert control panel at the beginning of container
        container.insertAdjacentHTML('afterbegin', controlPanelHTML);

        // Hide the releases table initially
        this.hideReleasesTable();
    }

    renderClientsTable(clientsEnvironments) {
        const pingStatuses = this.clientsEnvironments?.ping_statuses || {};
        const isAdminKey = this.config?.api_key_type?.is_admin || false;
        const authenticatedClient = this.config?.api_key_type?.authenticated_client || '';

        return Object.entries(clientsEnvironments).map(([clientName, environments]) => {
            // For standard API keys, only show the authenticated client
            if (!isAdminKey && authenticatedClient && clientName !== authenticatedClient) {
                return '';
            }

            // Determine overall client status (worst status among environments)
            let clientStatus = 'never';
            let latestPing = null;

            environments.forEach(env => {
                const envPingStatus = pingStatuses[clientName]?.[env];
                const envStatus = envPingStatus?.status;
                const envLastPing = envPingStatus?.last_ping;

                if (envStatus) {
                    if (envStatus === 'offline' || clientStatus === 'never') {
                        clientStatus = envStatus;
                    } else if (envStatus === 'warning' && clientStatus === 'online') {
                        clientStatus = envStatus;
                    } else if (envStatus === 'online' && clientStatus === 'never') {
                        clientStatus = envStatus;
                    }
                }

                // Track the latest ping time across all environments
                if (envLastPing && (!latestPing || new Date(envLastPing) > new Date(latestPing))) {
                    latestPing = envLastPing;
                }
            });

            // For standard API keys with a specific client, auto-select and show environments only
            const isClientPreselected = !isAdminKey && authenticatedClient === clientName;
            const rowClass = isClientPreselected ? 'client-row selected' : 'client-row';

            return `
                <tr class="${rowClass}" data-client="${clientName}" onclick="dashboard.selectClient('${clientName}')">
                    <td class="client-name-cell">
                        <div class="client-name-content">
                            <div class="status-indicator ${clientStatus}"
                                 onmouseenter="dashboard.showStatusTooltip(event, '${clientName}', 'client')"
                                 onmouseleave="dashboard.hideStatusTooltip()"></div>
                            <span class="client-name-text">${clientName}</span>
                            ${isClientPreselected ? '<span class="client-locked-indicator">🔒</span>' : ''}
                        </div>
                    </td>
                    <td class="env-count-cell">
                        <span class="env-count-badge">${environments.length}</span>
                    </td>
                    <td class="status-cell">
                        <span class="status-text ${clientStatus}">${this.getStatusText(clientStatus)}</span>
                    </td>
                    <td class="last-ping-cell">
                        <span class="ping-time">${latestPing ? this.formatTimestamp(latestPing) : 'Never'}</span>
                    </td>
                    <td class="environments-cell">
                        <div class="environments-list">
                            ${environments.map(env => {
                                const envPingStatus = pingStatuses[clientName]?.[env];
                                const envStatus = envPingStatus?.status || 'never';
                                const lastPing = envPingStatus?.last_ping;

                                return `
                                    <span class="env-tag" data-env="${env}"
                                          onclick="event.stopPropagation(); dashboard.selectEnvironment('${clientName}', '${env}')">
                                        ${env}
                                        <div class="env-status-dot ${envStatus}"
                                             onmouseenter="dashboard.showStatusTooltip(event, '${clientName}', '${env}', '${envStatus}', '${lastPing || ''}')"
                                             onmouseleave="dashboard.hideStatusTooltip()"></div>
                                    </span>
                                `;
                            }).join('')}
                            <button class="compare-btn" onclick="dashboard.openCompare()" style="display: none;">🔄</button>
                        </div>
                    </td>
                </tr>
            `;
        }).join('');
    }

    selectClient(clientName) {
        // For standard API keys, prevent changing the client
        const isAdminKey = this.config?.api_key_type?.is_admin || false;
        const authenticatedClient = this.config?.api_key_type?.authenticated_client || '';

        if (!isAdminKey && authenticatedClient && clientName !== authenticatedClient) {
            console.warn('Cannot select different client with standard API key');
            return;
        }

        this.selectedClient = clientName;
        this.selectedEnvironment = null;
        this.updateClientSelection();
        this.highlightEnvironmentStep();
        this.showNoEnvironmentMessage();
        // Clear stored environment since client changed
        localStorage.removeItem('release-tracker-environment');
        if (this.selectedClient) {
            localStorage.setItem('release-tracker-client', this.selectedClient);
        }
    }

    selectEnvironment(clientName, envName) {
        this.selectedClient = clientName;
        this.selectedEnvironment = envName;
        this.clearHighlights();
        this.updateClientSelection();
        this.loadMasterFilteredReleases();
        // Save both client and environment to storage
        this.saveSelectionToStorage();
    }

    updateClientSelection() {
        // Update visual selection
        document.querySelectorAll('.client-row').forEach(row => {
            row.classList.remove('selected');
        });
        document.querySelectorAll('.env-tag').forEach(tag => {
            tag.classList.remove('selected');
        });

        if (this.selectedClient) {
            const clientRow = document.querySelector(`[data-client="${this.selectedClient}"]`);
            if (clientRow) {
                clientRow.classList.add('selected');
            }
        }

        if (this.selectedEnvironment) {
            const envTag = document.querySelector(`[data-env="${this.selectedEnvironment}"]`);
            if (envTag) {
                envTag.classList.add('selected');
            }
        }

        // Update step indicator
        this.updateStepIndicator();

        // Update selection display
        this.updateSelectionDisplay();
        this.updateCompareButtonVisibility();
    }

    updateStepIndicator() {
        const step1 = document.getElementById('step1');
        const step2 = document.getElementById('step2');

        if (!step1 || !step2) return;

        // Reset steps
        step1.classList.remove('active', 'completed');
        step2.classList.remove('active', 'completed');

        if (this.selectedClient && this.selectedEnvironment) {
            // Both steps completed
            step1.classList.add('completed');
            step2.classList.add('completed');
        } else if (this.selectedClient) {
            // Step 1 completed, step 2 active
            step1.classList.add('completed');
            step2.classList.add('active');
        } else {
            // Step 1 active
            step1.classList.add('active');
        }
    }

    updateSelectionDisplay() {
        const selectionDiv = document.getElementById('currentSelection');
        const selectionText = document.getElementById('selectionText');
        const noSelectionState = document.getElementById('noSelectionState');

        if (this.selectedClient && this.selectedEnvironment) {
            selectionText.textContent = `${this.selectedClient} - ${this.selectedEnvironment}`;
            selectionDiv.style.display = 'block';
            noSelectionState.style.display = 'none';
        } else {
            selectionDiv.style.display = 'none';
            noSelectionState.style.display = 'block';
        }
    }

    clearSelection() {
        this.selectedClient = null;
        this.selectedEnvironment = null;
        this.clearHighlights();
        this.updateClientSelection();
        this.expandSelector();
        this.hideReleasesTable();
        this.showHeader(false);
        // Clear stored selections
        this.clearStoredSelection();
    }

    collapseSelector() {
        const controlPanel = document.getElementById('controlPanel');
        const clientsTableContainer = document.getElementById('clientsTableContainer');
        const selectorHeader = document.querySelector('.selector-header');

        if (controlPanel) {
            controlPanel.classList.add('collapsed');
        }

        if (clientsTableContainer) {
            clientsTableContainer.style.display = 'none';
        }

        if (selectorHeader) {
            const title = selectorHeader.querySelector('.selector-title');
            if (title) {
                title.textContent = 'Client: ' + this.selectedClient.toUpperCase() + ' - Environment: ' + this.selectedEnvironment.toUpperCase();
            }
        }

        // Show compare button when client is selected
        this.updateCompareButtonVisibility();
    }

    expandSelector() {
        const controlPanel = document.getElementById('controlPanel');
        const clientsTableContainer = document.getElementById('clientsTableContainer');
        const selectorHeader = document.querySelector('.selector-header');

        if (controlPanel) {
            controlPanel.classList.remove('collapsed');
        }

        if (clientsTableContainer) {
            clientsTableContainer.style.display = 'block';
        }

        if (selectorHeader) {
            const title = selectorHeader.querySelector('.selector-title');
            if (title) {
                title.textContent = 'Select Client & Environment';
            }
        }
    }

    highlightEnvironmentStep() {
        // Remove previous highlights
        document.querySelectorAll('.step-highlight').forEach(el => {
            el.classList.remove('step-highlight');
        });

        // Highlight environment tags for the selected client
        if (this.selectedClient) {
            const clientRow = document.querySelector(`[data-client="${this.selectedClient}"]`);
            if (clientRow) {
                const envTags = clientRow.querySelectorAll('.env-tag');
                envTags.forEach(tag => {
                    tag.classList.add('step-highlight');
                });
            }
        }
    }

    clearHighlights() {
        document.querySelectorAll('.step-highlight').forEach(el => {
            el.classList.remove('step-highlight');
        });
    }

    showStatusTooltip(event, clientName, envOrType, status, lastPing) {
        // Remove existing tooltip
        this.hideStatusTooltip();

        let tooltipText;
        if (envOrType === 'client') {
            tooltipText = `Client: ${clientName}`;
        } else {
            const statusText = this.getStatusText(status);
            tooltipText = `${clientName}/${envOrType}: ${statusText}`;
            if (lastPing && lastPing !== 'undefined') {
                const pingDate = new Date(lastPing);
                tooltipText += `\nLast ping: ${pingDate.toLocaleString()}`;
            }
        }

        const tooltip = document.createElement('div');
        tooltip.className = 'status-tooltip visible';
        tooltip.textContent = tooltipText;
        tooltip.style.whiteSpace = 'pre-line';

        document.body.appendChild(tooltip);

        // Position tooltip
        const rect = event.target.getBoundingClientRect();
        tooltip.style.left = (rect.left + rect.width / 2 - tooltip.offsetWidth / 2) + 'px';
        tooltip.style.top = (rect.top - tooltip.offsetHeight - 10) + 'px';

        this.currentTooltip = tooltip;
    }

    hideStatusTooltip() {
        if (this.currentTooltip) {
            this.currentTooltip.remove();
            this.currentTooltip = null;
        }
    }

    getStatusText(status) {
        switch (status) {
            case 'online': return 'Online';
            case 'warning': return 'Warning';
            case 'offline': return 'Offline';
            case 'never': return 'Never pinged';
            default: return 'Unknown';
        }
    }

    hideControlPanel() {
        const controlPanel = document.getElementById('controlPanel');
        if (controlPanel) {
            controlPanel.remove();
        }
    }

    hideReleasesTable() {
        const releasesSection = document.querySelector('.releases-section');
        if (releasesSection) {
            releasesSection.style.display = 'none';
        }
    }

    showReleasesTable() {
        const releasesSection = document.querySelector('.releases-section');
        if (releasesSection) {
            releasesSection.style.display = 'block';
        }
    }

    showNoEnvironmentMessage() {
        this.hideReleasesTable();
        // Could add a specific message for client selected but no environment
    }

    async loadMasterFilteredReleases() {
        if (!this.selectedClient || !this.selectedEnvironment) {
            this.hideReleasesTable();
            return;
        }

        try {
            const params = new URLSearchParams();
            params.append('client_name', this.selectedClient);
            params.append('env_name', this.selectedEnvironment);

            const url = `${this.basePath}/api/releases/current?${params}`;
            const response = await fetch(url, this.getFetchOptions());

            if (!response.ok) {
                this.handleAuthError(null, response);
                return;
            }

            const data = await response.json();

            // Extract releases from the nested structure
            let releasesArray = [];
            if (Array.isArray(data)) {
                releasesArray = data;
            } else if (data && data.namespaces) {
                releasesArray = [];
                Object.values(data.namespaces).forEach(namespaceReleases => {
                    if (Array.isArray(namespaceReleases)) {
                        releasesArray.push(...namespaceReleases);
                    }
                });
            } else if (data && data.ordered_namespaces) {
                releasesArray = data.ordered_namespaces || [];
            }

            this.releases = releasesArray;
            this.filteredReleases = releasesArray;

            // Show header
            this.showHeader(true);

            this.processReleasesData(data);
            // Update stats for filtered data
            this.updateStatsForFiltered(releasesArray, data.timestamp);

            // Collapse the selector and show results
            this.collapseSelector();

            this.renderTable();
            this.showReleasesTable();

            if (releasesArray.length === 0) {
                this.showEmptyState();
            }
        } catch (error) {
            console.error('Failed to load filtered releases:', error);
            this.showError('Failed to load releases. Please try again.');
        }
    }

    updateStatsForFiltered(releases, timestamp) {
        if (!Array.isArray(releases)) {
            releases = [];
        }

        document.getElementById('totalReleases').textContent = releases.length;

        const namespaces = new Set(releases.map(r => r.namespace));
        document.getElementById('totalNamespaces').textContent = namespaces.size;

        document.getElementById('lastUpdated').textContent = this.formatTimestamp(timestamp);
    }

    showEmptyState() {
        const emptyState = document.getElementById('emptyState');
        if (emptyState) {
            const emptyStateContent = emptyState.querySelector('h3');
            const emptyStateDesc = emptyState.querySelector('p');

            if (emptyStateContent) {
                emptyStateContent.textContent = `No releases found for ${this.selectedClient} - ${this.selectedEnvironment}`;
            }
            if (emptyStateDesc) {
                emptyStateDesc.textContent = 'This client/environment combination has no recorded releases yet.';
            }

            emptyState.style.display = 'block';
        }
    }

    bindEvents() {
        // Search functionality
        document.getElementById('searchInput').addEventListener('input', (e) => {
            this.filterReleases(e.target.value);
        });

        document.getElementById('clearSearch').addEventListener('click', () => {
            document.getElementById('searchInput').value = '';
            this.filterReleases('');
        });

        // Action buttons
        document.getElementById('collectBtn').addEventListener('click', () => {
            this.triggerCollection();
        });

        // Error dismissal
        document.getElementById('dismissError').addEventListener('click', () => {
            this.hideError();
        });

        // Success dismissal
        document.getElementById('dismissSuccess').addEventListener('click', () => {
            this.hideSuccess();
        });
    }

    async loadSlaveFilteredReleases() {
        // Wait for config to load first
        if (!this.config) {
            // Config not loaded yet, wait a bit and retry
            setTimeout(() => this.loadSlaveFilteredReleases(), 100);
            return;
        }
        // In master mode, don't load releases automatically
        if (this.config && this.config.mode === 'master') {
            return;
        }

        this.showLoading();
        this.hideError();
        this.hideSuccess();

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
            this.processReleasesData(data);
            this.updateStats(data);
            this.renderTable();
        } catch (error) {
            console.error('Failed to load releases:', error);
            if (!this.handleAuthError(error, null)) {
                this.showError(`Failed to load releases: ${error.message}`);
            }
        } finally {
            this.hideLoading();
        }
    }

    processReleasesData(data) {
        this.releases = [];
        this.namespaceOrder = [];

        // Use ordered namespaces if available, otherwise fall back to regular namespaces
        if (data.ordered_namespaces) {
            data.ordered_namespaces.forEach(nsData => {
                this.namespaceOrder.push(nsData.name);
                nsData.releases.forEach(release => {
                    this.releases.push({
                        ...release,
                        namespace: nsData.name
                    });
                });
            });
        } else if (data.namespaces) {
            // Fallback for backward compatibility
            Object.entries(data.namespaces).forEach(([namespace, releases]) => {
                this.namespaceOrder.push(namespace);
                releases.forEach(release => {
                    this.releases.push({
                        ...release,
                        namespace: namespace
                    });
                });
            });
        }

        // Sort by namespace order, then workload name, then container name
        this.releases.sort((a, b) => {
            const aIndex = this.namespaceOrder.indexOf(a.namespace);
            const bIndex = this.namespaceOrder.indexOf(b.namespace);

            if (aIndex !== bIndex) return aIndex - bIndex;
            if (a.workload_name !== b.workload_name) return a.workload_name.localeCompare(b.workload_name);
            return a.container_name.localeCompare(b.container_name);
        });

        this.filteredReleases = [...this.releases];
    }

    updateStats(data) {
        document.getElementById('totalReleases').textContent = data.total || 0;
        document.getElementById('totalNamespaces').textContent = Object.keys(data.namespaces || {}).length;
        document.getElementById('lastUpdated').textContent = this.formatTimestamp(data.timestamp);
    }

    renderTable() {
        const tbody = document.getElementById('releasesTableBody');
        const emptyState = document.getElementById('emptyState');
        const tableContainer = document.querySelector('.table-container');

        if (this.filteredReleases.length === 0) {
            tableContainer.style.display = 'none';
            emptyState.style.display = 'block';
            return;
        }

        tableContainer.style.display = 'block';
        emptyState.style.display = 'none';

        // Group releases by namespace, then by workload
        const groupedReleases = {};
        this.filteredReleases.forEach(release => {
            if (!groupedReleases[release.namespace]) {
                groupedReleases[release.namespace] = {};
            }
            const workloadKey = `${release.workload_type}:${release.workload_name}`;
            if (!groupedReleases[release.namespace][workloadKey]) {
                groupedReleases[release.namespace][workloadKey] = [];
            }
            groupedReleases[release.namespace][workloadKey].push(release);
        });

        let html = '';
        let isFirstNamespace = true;

        // Use namespace order from configuration
        const namespacesToShow = this.namespaceOrder.filter(ns => groupedReleases[ns]);

        namespacesToShow.forEach(namespace => {
            const workloads = groupedReleases[namespace];

            // Add namespace header row
            html += `
                <tr class="namespace-group ${isFirstNamespace ? 'first' : ''}">
                    <td class="namespace-cell" colspan="10">
                        📁 ${this.escapeHtml(namespace)}
                    </td>
                </tr>
            `;

            let isFirstWorkload = true;

            // Sort workloads by name
            const sortedWorkloadKeys = Object.keys(workloads).sort((a, b) => {
                const [, nameA] = a.split(':');
                const [, nameB] = b.split(':');
                return nameA.localeCompare(nameB);
            });

            sortedWorkloadKeys.forEach(workloadKey => {
                const releases = workloads[workloadKey];
                const [workloadType, workloadName] = workloadKey.split(':');

                // Add releases for this workload
                releases.forEach((release, index) => {
                    if (index === 0) {
                        // First container row includes workload info
                        html += `
                            <tr class="workload-group ${isFirstWorkload ? 'first' : ''}">
                                <td></td>
                                <td>
                                    <span class="workload-type-cell workload-type-${workloadType.toLowerCase()}">${this.escapeHtml(workloadType)}</span>
                                </td>
                                <td class="workload-name-cell">${this.escapeHtml(workloadName)}</td>
                                <td>${this.escapeHtml(release.container_name)}</td>
                                <td>${this.escapeHtml(release.image_repo || '-')}</td>
                                <td>${this.escapeHtml(release.image_name)}</td>
                                <td>
                                    <span class="image-tag-cell">${this.escapeHtml(release.image_tag)}</span>
                                </td>
                                <td>
                                    <span class="image-sha-cell" title="${this.escapeHtml(release.image_sha || '')}">${this.formatImageSHA(release.image_sha)}</span>
                                </td>
                                <td class="timestamp-cell">${this.formatTimestamp(release.last_seen)}</td>
                                <td class="actions-cell">
                                    <a href="${this.basePath}/timeline.html?namespace=${encodeURIComponent(release.namespace)}&workload=${encodeURIComponent(release.workload_name)}&container=${encodeURIComponent(release.container_name)}&apikey=${encodeURIComponent(this.apiKey)}&client=${encodeURIComponent(this.selectedClient)}&env=${encodeURIComponent(this.selectedEnvironment)}"
                                       class="view-history-btn">History</a>
                                </td>
                            </tr>
                        `;
                    } else {
                        // Subsequent container rows are indented
                        html += `
                            <tr>
                                <td></td>
                                <td></td>
                                <td></td>
                                <td>${this.escapeHtml(release.container_name)}</td>
                                <td>${this.escapeHtml(release.image_repo || '-')}</td>
                                <td>${this.escapeHtml(release.image_name)}</td>
                                <td>
                                    <span class="image-tag-cell">${this.escapeHtml(release.image_tag)}</span>
                                </td>
                                <td>
                                    <span class="image-sha-cell" title="${this.escapeHtml(release.image_sha || '')}">${this.formatImageSHA(release.image_sha)}</span>
                                </td>
                                <td class="timestamp-cell">${this.formatTimestamp(release.last_seen)}</td>
                                <td class="actions-cell">
                                    <a href="${this.basePath}/timeline.html?namespace=${encodeURIComponent(release.namespace)}&workload=${encodeURIComponent(release.workload_name)}&container=${encodeURIComponent(release.container_name)}&apikey=${encodeURIComponent(this.apiKey)}&client=${encodeURIComponent(this.selectedClient)}&env=${encodeURIComponent(this.selectedEnvironment)}"
                                       class="view-history-btn">History</a>
                                </td>
                            </tr>
                        `;
                    }
                });

                isFirstWorkload = false;
            });

            isFirstNamespace = false;
        });

        tbody.innerHTML = html;
    }

    filterReleases(searchTerm) {
        if (!searchTerm.trim()) {
            this.filteredReleases = [...this.releases];
        } else {
            const term = searchTerm.toLowerCase();
            this.filteredReleases = this.releases.filter(release =>
                release.namespace.toLowerCase().includes(term) ||
                release.workload_name.toLowerCase().includes(term) ||
                release.workload_type.toLowerCase().includes(term) ||
                release.container_name.toLowerCase().includes(term) ||
                release.image_repo.toLowerCase().includes(term) ||
                release.image_name.toLowerCase().includes(term) ||
                release.image_tag.toLowerCase().includes(term)
            );
        }
        this.renderTable();
    }

    async triggerCollection() {
        const collectBtn = document.getElementById('collectBtn');
        const originalText = collectBtn.textContent;

        collectBtn.disabled = true;
        collectBtn.textContent = 'Starting...';
        this.hideError();
        this.hideSuccess();

        try {
            const response = await fetch(`${this.basePath}/api/collect`, this.getFetchOptions({
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                }
            }));

            if (!response.ok) {
                if (this.handleAuthError(null, response)) {
                    return;
                }
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }

            const result = await response.json();
            console.log('Collection result:', result);

            // Check if the collection was accepted (async response)
            if (result.status === 'accepted') {
                this.showSuccess('Background collection initiated successfully. Data will be updated automatically in a few moments.');

                // Reload releases after a longer delay to account for background processing
                setTimeout(() => this.loadSlaveFilteredReleases(), 30000);
            }

        } catch (error) {
            console.error('Collection failed:', error);
            if (!this.handleAuthError(error, null)) {
                this.showError(`Collection failed: ${error.message}`);
            }
        } finally {
            collectBtn.disabled = false;
            collectBtn.textContent = originalText;
        }
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

    showSuccess(message) {
        document.getElementById('successMessage').textContent = message;
        document.getElementById('success').style.display = 'flex';
    }

    hideSuccess() {
        document.getElementById('success').style.display = 'none';
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

    escapeHtml(text) {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// Initialize dashboard when DOM is loaded
let dashboard;
document.addEventListener('DOMContentLoaded', () => {
    dashboard = new Dashboard();
});
