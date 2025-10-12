class BadgeSummary {
    constructor() {
        this.apiKey = this.extractAPIKey();
        this.basePath = this.getBasePath();
        this.config = null;
        this.currentSummaryData = null;
        this.workloadCounter = 1;
        this.currentExportType = 'markdown';
        this.init();
    }

    // Get base path from current URL
    getBasePath() {
        const path = window.location.pathname;
        const segments = path.split('/').filter(s => s);

        // If we're at a specific page, remove the filename
        if (segments.length > 0 && segments[segments.length - 1].includes('.html')) {
            segments.pop();
        }

        // Return base path or empty string
        return segments.length > 0 ? '/' + segments.join('/') : '';
    }

    init() {
        this.bindEvents();
        this.loadConfig();
        this.loadURLParameters();
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
        document.getElementById('summaryForm').addEventListener('submit', (e) => {
            e.preventDefault();
            this.generateSummary();
        });

        document.getElementById('clearBtn').addEventListener('click', () => {
            this.clearForm();
        });

        document.getElementById('addWorkloadBtn').addEventListener('click', () => {
            this.addWorkload();
        });

        // Export tab switching
        document.querySelectorAll('.export-tab').forEach(tab => {
            tab.addEventListener('click', (e) => {
                this.switchExportTab(e.target.dataset.tab);
            });
        });

        document.getElementById('copyExportBtn').addEventListener('click', () => {
            this.copyExport();
        });

        // Event delegation for remove workload buttons
        document.getElementById('workloadsContainer').addEventListener('click', (e) => {
            if (e.target.classList.contains('remove-workload-btn')) {
                this.removeWorkload(e.target.closest('.workload-entry'));
            }
        });
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
    }

    addWorkload() {
        this.workloadCounter++;
        const container = document.getElementById('workloadsContainer');
        const workloadEntry = document.createElement('div');
        workloadEntry.className = 'workload-entry';
        workloadEntry.dataset.workloadIndex = this.workloadCounter - 1;

        workloadEntry.innerHTML = `
            <div class="workload-header">
                <span class="workload-number">Workload ${this.workloadCounter}</span>
                <button type="button" class="remove-workload-btn">Remove</button>
            </div>
            <div class="form-row">
                <div class="form-group">
                    <label>Workload Kind:</label>
                    <select name="workloadKind" required>
                        <option value="">Select workload type...</option>
                        <option value="Deployment">Deployment</option>
                        <option value="StatefulSet">StatefulSet</option>
                        <option value="DaemonSet">DaemonSet</option>
                        <option value="Other">Other</option>
                    </select>
                </div>

                <div class="form-group">
                    <label>Workload Name:</label>
                    <input type="text" name="workloadName" placeholder="e.g., web-app, api-server" required>
                </div>

                <div class="form-group">
                    <label>Container Name:</label>
                    <input type="text" name="containerName" placeholder="e.g., web, api, worker" required>
                </div>
            </div>
        `;

        container.appendChild(workloadEntry);
    }

    removeWorkload(workloadEntry) {
        const container = document.getElementById('workloadsContainer');
        if (container.children.length > 1) {
            workloadEntry.remove();
            this.updateWorkloadNumbers();
        }
    }

    updateWorkloadNumbers() {
        const workloadEntries = document.querySelectorAll('.workload-entry');
        workloadEntries.forEach((entry, index) => {
            const numberSpan = entry.querySelector('.workload-number');
            if (numberSpan) {
                numberSpan.textContent = `Workload ${index + 1}`;
            }
        });
        this.workloadCounter = workloadEntries.length;
    }

    async generateSummary() {
        const clientName = document.getElementById('clientName').value.trim();
        const environmentsText = document.getElementById('environments').value.trim();

        if (!clientName || !environmentsText) {
            this.showError('Please fill in client name and environments.');
            return;
        }

        if (!this.apiKey) {
            this.showError('API key is required. Please add ?apikey=YOUR_API_KEY to the URL.');
            return;
        }

        // Parse environments
        const environments = environmentsText.split(',').map(env => env.trim()).filter(env => env);
        if (environments.length === 0) {
            this.showError('Please enter at least one environment.');
            return;
        }

        // Collect workload data
        const workloads = [];
        const workloadEntries = document.querySelectorAll('.workload-entry');

        for (const entry of workloadEntries) {
            const workloadKind = entry.querySelector('select[name="workloadKind"]').value.trim();
            const workloadName = entry.querySelector('input[name="workloadName"]').value.trim();
            const containerName = entry.querySelector('input[name="containerName"]').value.trim();

            if (!workloadKind || !workloadName || !containerName) {
                this.showError('Please fill in all workload fields.');
                return;
            }

            workloads.push({
                workloadKind,
                workloadName,
                containerName
            });
        }

        if (workloads.length === 0) {
            this.showError('Please add at least one workload.');
            return;
        }

        this.hideMessages();
        this.showLoading();

        try {
            // Store current summary data
            this.currentSummaryData = {
                clientName,
                environments,
                workloads
            };

            // Generate the summary table
            this.renderSummaryTable();
            this.generateExports();
            this.showSummaryOutput();

            this.showInfo(`Badge summary generated successfully for ${workloads.length} workload(s) across ${environments.length} environment(s)!`);
        } catch (error) {
            console.error('Failed to generate summary:', error);
            this.showError(`Failed to generate summary: ${error.message}`);
        } finally {
            this.hideLoading();
        }
    }

    renderSummaryTable() {
        const { clientName, environments, workloads } = this.currentSummaryData;

        let tableHTML = `
            <table class="summary-table">
                <tbody>
                    ${workloads.map(workload => `
                        <tr>
                            <td class="workload-info">
                                <strong>${this.escapeHtml(workload.workloadKind)}/${this.escapeHtml(workload.workloadName)}</strong><br>
                                <small>Container: ${this.escapeHtml(workload.containerName)}</small><br>
                                <small>Client: ${this.escapeHtml(clientName)}</small>
                            </td>
                            ${environments.map(env => this.renderBadgeCell(clientName, env, workload.workloadKind, workload.workloadName, workload.containerName)).join('')}
                        </tr>
                    `).join('')}
                </tbody>
            </table>
        `;

        document.getElementById('summaryTableContainer').innerHTML = tableHTML;
    }

    renderBadgeCell(clientName, envName, workloadKind, workloadName, containerName) {
        const badgeUrl = `${this.basePath}/badges/${encodeURIComponent(this.apiKey)}/${encodeURIComponent(clientName)}/${encodeURIComponent(envName)}/${encodeURIComponent(workloadKind)}/${encodeURIComponent(workloadName)}/${encodeURIComponent(containerName)}`;

        return `
            <td class="badge-cell">
                <img src="${badgeUrl}" alt="Release Badge for ${this.escapeHtml(envName)}"
                     onerror="this.style.display='none'; this.nextElementSibling.style.display='block';">
                <div style="display: none; color: #dc3545; font-size: 12px;">
                    Badge unavailable
                </div>
            </td>
        `;
    }

    generateExports() {
        this.generateMarkdown();
        this.generateHTML();
        this.updateExportDisplay();
    }

    generateMarkdown() {
        const { clientName, environments, workloads } = this.currentSummaryData;
        const baseUrl = window.location.origin + this.basePath;

        let markdown = `# Deployment Status Summary\n\n`;
        markdown += `| Workload Name | ${environments.join(' | ')} |\n`;
        markdown += `|${'-'.repeat(15)}|${environments.map(() => '-'.repeat(17)).join('|')}|\n`;

        workloads.forEach(workload => {
            const badgeCells = environments.map(env => {
                const badgeUrl = `${baseUrl}/badges/${encodeURIComponent(this.apiKey)}/${encodeURIComponent(clientName)}/${encodeURIComponent(env)}/${encodeURIComponent(workload.workloadKind)}/${encodeURIComponent(workload.workloadName)}/${encodeURIComponent(workload.containerName)}`;
                return `![${env}](${badgeUrl})`;
            }).join(' | ');

            markdown += `| ${workload.workloadKind}/${workload.workloadName} | ${badgeCells} |\n`;
        });

        markdown += `\n*Client: ${clientName}*\n`;

        this.markdownContent = markdown;
    }

    generateHTML() {
        const { clientName, environments, workloads } = this.currentSummaryData;
        const baseUrl = window.location.origin + this.basePath;

        let html = `<table border="1" cellpadding="8" cellspacing="0" style="border-collapse: collapse; width: 100%;">\n`;
        html += `  <thead>\n`;
        html += `    <tr style="background-color: #f8f9fa;">\n`;
        html += `      <th style="text-align: left; font-weight: bold;">Workload Name</th>\n`;
        environments.forEach(env => {
            html += `      <th style="text-align: center; font-weight: bold;">${this.escapeHtml(env)}</th>\n`;
        });
        html += `    </tr>\n`;
        html += `  </thead>\n`;
        html += `  <tbody>\n`;

        workloads.forEach(workload => {
            html += `    <tr>\n`;
            html += `      <td style="font-weight: bold;">\n`;
            html += `        ${this.escapeHtml(workload.workloadKind)}/${this.escapeHtml(workload.workloadName)}<br>\n`;
            html += `        <small>Container: ${this.escapeHtml(workload.containerName)}</small><br>\n`;
            html += `        <small>Client: ${this.escapeHtml(clientName)}</small>\n`;
            html += `      </td>\n`;

            environments.forEach(env => {
                const badgeUrl = `${baseUrl}/badges/${encodeURIComponent(this.apiKey)}/${encodeURIComponent(clientName)}/${encodeURIComponent(env)}/${encodeURIComponent(workload.workloadKind)}/${encodeURIComponent(workload.workloadName)}/${encodeURIComponent(workload.containerName)}`;
                html += `      <td style="text-align: center;">\n`;
                html += `        <img src="${badgeUrl}" alt="Release Badge for ${this.escapeHtml(env)}" style="max-width: 100%; height: auto;">\n`;
                html += `      </td>\n`;
            });

            html += `    </tr>\n`;
        });

        html += `  </tbody>\n`;
        html += `</table>`;

        this.htmlContent = html;
    }

    switchExportTab(tabType) {
        this.currentExportType = tabType;

        // Update tab appearance
        document.querySelectorAll('.export-tab').forEach(tab => {
            tab.classList.remove('active');
        });
        document.querySelector(`[data-tab="${tabType}"]`).classList.add('active');

        // Update export content and button
        this.updateExportDisplay();
    }

    updateExportDisplay() {
        const exportContent = document.getElementById('exportContent');
        const copyBtn = document.getElementById('copyExportBtn');

        if (this.currentExportType === 'markdown') {
            exportContent.textContent = this.markdownContent || '';
            copyBtn.textContent = 'Copy Markdown';
        } else {
            exportContent.textContent = this.htmlContent || '';
            copyBtn.textContent = 'Copy HTML';
        }
    }

    copyExport() {
        const content = this.currentExportType === 'markdown' ? this.markdownContent : this.htmlContent;

        if (!content) {
            this.showError('No content to copy. Please generate a summary first.');
            return;
        }

        navigator.clipboard.writeText(content).then(() => {
            const button = document.getElementById('copyExportBtn');
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

    clearForm() {
        document.getElementById('summaryForm').reset();
        document.getElementById('summaryOutput').style.display = 'none';
        this.hideMessages();
        this.currentSummaryData = null;
        this.markdownContent = '';
        this.htmlContent = '';

        // Reset to single workload
        const container = document.getElementById('workloadsContainer');
        container.innerHTML = `
            <div class="workload-entry" data-workload-index="0">
                <div class="workload-header">
                    <span class="workload-number">Workload 1</span>
                </div>
                <div class="form-row">
                    <div class="form-group">
                        <label>Workload Kind:</label>
                        <select name="workloadKind" required>
                            <option value="">Select workload type...</option>
                            <option value="Deployment">Deployment</option>
                            <option value="StatefulSet">StatefulSet</option>
                            <option value="DaemonSet">DaemonSet</option>
                            <option value="Other">Other</option>
                        </select>
                    </div>

                    <div class="form-group">
                        <label>Workload Name:</label>
                        <input type="text" name="workloadName" placeholder="e.g., web-app, api-server" required>
                    </div>

                    <div class="form-group">
                        <label>Container Name:</label>
                        <input type="text" name="containerName" placeholder="e.g., web, api, worker" required>
                    </div>
                </div>
            </div>
        `;
        this.workloadCounter = 1;

        // Restore config placeholders
        this.updateFormPlaceholders();
    }

    showSummaryOutput() {
        document.getElementById('summaryOutput').style.display = 'block';
        // Scroll to the output
        document.getElementById('summaryOutput').scrollIntoView({ behavior: 'smooth' });
    }

    showLoading() {
        document.getElementById('loading').style.display = 'block';
    }

    hideLoading() {
        document.getElementById('loading').style.display = 'none';
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

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    // Load parameters from URL if present
    loadURLParameters() {
        const urlParams = new URLSearchParams(window.location.search);
        const client = urlParams.get('client');
        const workloadKind = urlParams.get('workload-kind');
        const workloadName = urlParams.get('workload-name');
        const container = urlParams.get('container');
        const environments = urlParams.get('environments');

        if (client) {
            document.getElementById('clientName').value = client;
        }
        if (environments) {
            document.getElementById('environments').value = environments;
        }

        // If workload parameters are provided, fill the first workload entry
        if (workloadKind || workloadName || container) {
            const firstWorkload = document.querySelector('.workload-entry');
            if (firstWorkload) {
                if (workloadKind) {
                    firstWorkload.querySelector('select[name="workloadKind"]').value = workloadKind;
                }
                if (workloadName) {
                    firstWorkload.querySelector('input[name="workloadName"]').value = workloadName;
                }
                if (container) {
                    firstWorkload.querySelector('input[name="containerName"]').value = container;
                }
            }
        }
    }
}

// Initialize the badge summary when the page loads
document.addEventListener('DOMContentLoaded', () => {
    window.badgeSummary = new BadgeSummary();
});

// Make copyExport available globally for onclick handlers (if needed)
window.copyExport = () => {
    if (window.badgeSummary) {
        window.badgeSummary.copyExport();
    }
};
