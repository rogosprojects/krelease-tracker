class BadgeViewer {
    constructor() {
        this.apiKey = this.extractAPIKey();
        this.basePath = this.getBasePath();
        this.config = null;
        this.init();
    }

    // Local storage utility functions
    saveFormToStorage() {
        const formData = {
            clientName: document.getElementById('clientName').value.trim(),
            envName: document.getElementById('envName').value.trim(),
            workloadKind: document.getElementById('workloadKind').value.trim(),
            workloadName: document.getElementById('workloadName').value.trim(),
            containerName: document.getElementById('containerName').value.trim()
        };

        // Only save if at least client and environment are filled
        if (formData.clientName && formData.envName) {
            localStorage.setItem('release-tracker-badge-form', JSON.stringify(formData));
        }
    }

    loadFormFromStorage() {
        const savedData = localStorage.getItem('release-tracker-badge-form');
        if (savedData) {
            try {
                return JSON.parse(savedData);
            } catch (error) {
                console.warn('Failed to parse saved form data:', error);
                localStorage.removeItem('release-tracker-badge-form');
            }
        }
        return null;
    }

    clearStoredForm() {
        localStorage.removeItem('release-tracker-badge-form');
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
        this.loadURLParameters();
        this.restoreFormFromStorage();
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

    bindEvents() {
        document.getElementById('badgeForm').addEventListener('submit', (e) => {
            e.preventDefault();
            this.generateBadge();
        });

        // Bind clear button
        const clearBtn = document.querySelector('.btn-secondary');
        if (clearBtn) {
            clearBtn.addEventListener('click', () => this.clearForm());
        }

        // Save form data when inputs change
        const formInputs = ['clientName', 'envName', 'workloadKind', 'workloadName', 'containerName'];
        formInputs.forEach(inputId => {
            const input = document.getElementById(inputId);
            if (input) {
                input.addEventListener('input', () => this.saveFormToStorage());
            }
        });
    }

    // Update navigation links to preserve API key
    updateNavigationLinks() {
        if (this.apiKey) {
            const dashboardLink = document.querySelector('a[href="index.html"]');
            const timelineLink = document.querySelector('a[href="timeline.html"]');

            if (dashboardLink) {
                dashboardLink.href = `index.html?apikey=${encodeURIComponent(this.apiKey)}`;
            }
            if (timelineLink) {
                timelineLink.href = `timeline.html?apikey=${encodeURIComponent(this.apiKey)}`;
            }
        }
    }

    async loadConfig() {
        try {
            const response = await fetch(`${this.basePath}/api/config`, this.getFetchOptions());
            if (response.ok) {
                const config = await response.json();
                this.config = config;

                // Update form placeholders with config values
                this.updateFormPlaceholders();
            }
        } catch (error) {
            console.warn('Failed to load config:', error);
        }
    }

    updateFormPlaceholders() {
        if (!this.config) return;

        // Update client name placeholder
        const clientInput = document.getElementById('clientName');
        if (clientInput && this.config.client_name) {
            clientInput.placeholder = `e.g., ${this.config.client_name}`;
            clientInput.value = this.config.client_name;
        }

        // Update environment name placeholder
        const envInput = document.getElementById('envName');
        if (envInput && this.config.env_name) {
            envInput.placeholder = `e.g., ${this.config.env_name}`;
            envInput.value = this.config.env_name;
        }
    }

    generateBadge() {
        const workloadKind = document.getElementById('workloadKind').value.trim();
        const workloadName = document.getElementById('workloadName').value.trim();
        const containerName = document.getElementById('containerName').value.trim();
        const clientName = document.getElementById('clientName').value.trim();
        const envName = document.getElementById('envName').value.trim();

        if (!workloadKind || !workloadName || !containerName || !clientName || !envName) {
            this.showError('Please fill in all fields.');
            return;
        }

        if (!this.apiKey) {
            this.showError('API key is required for badge generation. Please add ?apikey=YOUR_API_KEY to the URL.');
            return;
        }

        this.hideMessages();

        // Use the new badge endpoint format with URL-based API key authentication
        const badgeUrl = `${this.basePath}/badges/${encodeURIComponent(this.apiKey)}/${encodeURIComponent(clientName)}/${encodeURIComponent(envName)}/${encodeURIComponent(workloadKind)}/${encodeURIComponent(workloadName)}/${encodeURIComponent(containerName)}`;
        const fullUrl = window.location.origin + badgeUrl;

        // Display the badge
        const badgeDisplay = document.getElementById('badgeDisplay');
        badgeDisplay.innerHTML = `<img src="${badgeUrl}" alt="Release Badge" onerror="window.badgeViewer.handleBadgeError()">`;

        // Display URLs
        document.getElementById('badgeUrl').textContent = fullUrl;
        document.getElementById('badgeMarkdown').textContent = `![Release Badge](${fullUrl})`;

        // Show the container
        document.getElementById('badgeContainer').style.display = 'block';

        this.showInfo('Badge generated successfully with authentication! You can copy the URL or Markdown code to embed in your README.');
    }

    handleBadgeError() {
        this.showError('Failed to load badge. Please check that the workload and container exist, and that your API key is valid.');
    }

    clearForm() {
        document.getElementById('badgeForm').reset();
        document.getElementById('badgeContainer').style.display = 'none';
        this.hideMessages();

        // Clear stored form data
        this.clearStoredForm();

        // Restore config placeholders
        this.updateFormPlaceholders();
    }

    restoreFormFromStorage() {
        // Only restore if URL parameters didn't already populate the form
        const urlParams = new URLSearchParams(window.location.search);
        const hasUrlParams = urlParams.get('workload-kind') || urlParams.get('client') || urlParams.get('env');

        if (!hasUrlParams) {
            const savedData = this.loadFormFromStorage();
            if (savedData) {
                // Restore form fields
                if (savedData.clientName) document.getElementById('clientName').value = savedData.clientName;
                if (savedData.envName) document.getElementById('envName').value = savedData.envName;
                if (savedData.workloadKind) document.getElementById('workloadKind').value = savedData.workloadKind;
                if (savedData.workloadName) document.getElementById('workloadName').value = savedData.workloadName;
                if (savedData.containerName) document.getElementById('containerName').value = savedData.containerName;
            }
        }
    }

    showError(message) {
        const errorDiv = document.getElementById('errorMessage');
        errorDiv.textContent = message;
        errorDiv.style.display = 'block';
        document.getElementById('infoMessage').style.display = 'none';
    }

    showInfo(message) {
        const infoDiv = document.getElementById('infoMessage');
        infoDiv.textContent = message;
        infoDiv.style.display = 'block';
        document.getElementById('errorMessage').style.display = 'none';
    }

    hideMessages() {
        document.getElementById('errorMessage').style.display = 'none';
        document.getElementById('infoMessage').style.display = 'none';
    }

    copyToClipboard(elementId) {
        const element = document.getElementById(elementId);
        const text = element.textContent;

        navigator.clipboard.writeText(text).then(() => {
            // Temporarily change button text to show success
            const button = element.parentElement.querySelector('.copy-btn');
            const originalText = button.textContent;
            button.textContent = 'Copied!';
            button.style.background = '#28a745';

            setTimeout(() => {
                button.textContent = originalText;
                button.style.background = '';
            }, 2000);
        }).catch((err) => {
            console.error('Failed to copy text: ', err);
            alert('Failed to copy to clipboard. Please copy manually.');
        });
    }

    // Load parameters from URL if present
    loadURLParameters() {
        const urlParams = new URLSearchParams(window.location.search);
        const workloadKind = urlParams.get('workload-kind');
        const workloadName = urlParams.get('workload-name');
        const container = urlParams.get('container');
        const client = urlParams.get('client');
        const env = urlParams.get('env');

        if (workloadKind && workloadName && container) {
            document.getElementById('workloadKind').value = workloadKind;
            document.getElementById('workloadName').value = workloadName;
            document.getElementById('containerName').value = container;

            if (client) {
                document.getElementById('clientName').value = client;
            }
            if (env) {
                document.getElementById('envName').value = env;
            }

            // Generate badge if all required fields are present
            if (client && env) {
                this.generateBadge();
            }
        }
    }
}

// Initialize the badge viewer when the page loads
document.addEventListener('DOMContentLoaded', () => {
    window.badgeViewer = new BadgeViewer();
});

// Make copyToClipboard available globally for onclick handlers
window.copyToClipboard = (elementId) => {
    if (window.badgeViewer) {
        window.badgeViewer.copyToClipboard(elementId);
    }
};
