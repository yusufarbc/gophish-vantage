/**
 * PHASE 1: UI/UX PERFECTION & MICRO-INTERACTIONS
 * Enterprise-grade UI components: Toast system, Skeleton loaders, Form validation
 */

// ═════════════════════════════════════════════════════════════════════════
// TOAST NOTIFICATION SYSTEM
// ═════════════════════════════════════════════════════════════════════════

class ToastNotification {
    constructor(message, type = 'info', duration = 4000) {
        this.message = message;
        this.type = type; // 'success', 'error', 'warning', 'info'
        this.duration = duration;
        this.el = null;
        this.create();
    }

    create() {
        const colors = {
            success: {
                bg: 'bg-emerald-900/90',
                border: 'border-emerald-700',
                icon: '<svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"/></svg>',
                text: 'text-emerald-300'
            },
            error: {
                bg: 'bg-red-900/90',
                border: 'border-red-700',
                icon: '<svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"/></svg>',
                text: 'text-red-300'
            },
            warning: {
                bg: 'bg-amber-900/90',
                border: 'border-amber-700',
                icon: '<svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z"/></svg>',
                text: 'text-amber-300'
            },
            info: {
                bg: 'bg-blue-900/90',
                border: 'border-blue-700',
                icon: '<svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z"/></svg>',
                text: 'text-blue-300'
            }
        };

        const style = colors[this.type] || colors.info;

        this.el = document.createElement('div');
        this.el.className = `
            ${style.bg} ${style.border} ${style.text}
            border rounded-lg px-4 py-3 shadow-lg backdrop-blur-sm
            flex items-center gap-3 max-w-md animate-in slide-in-from-right
            pointer-events-auto cursor-pointer transition-all hover:shadow-xl
        `.replace(/\s+/g, ' ').trim();

        this.el.innerHTML = `
            <span class="flex-shrink-0">${style.icon}</span>
            <span class="flex-1 text-sm font-medium">${this.escape(this.message)}</span>
            <button class="flex-shrink-0 hover:opacity-70 transition" onclick="this.closest('[data-toast]')?.remove()">
                <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"/>
                </svg>
            </button>
        `;

        this.el.setAttribute('data-toast', this.type);

        // Ensure container exists
        let container = document.getElementById('toast-container');
        if (!container) {
            container = document.createElement('div');
            container.id = 'toast-container';
            container.className = 'fixed bottom-4 right-4 space-y-3 z-[9999] pointer-events-none';
            document.body.appendChild(container);
        }
        container.appendChild(this.el);

        if (this.duration > 0) {
            setTimeout(() => this.dismiss(), this.duration);
        }
    }

    dismiss() {
        if (this.el) {
            this.el.style.animation = 'slide-out-to-right 0.3s ease-in-out forwards';
            setTimeout(() => this.el?.remove(), 300);
        }
    }

    escape(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// Global toast functions
window.showToast = function (message, type = 'info', duration = 4000) {
    new ToastNotification(message, type, duration);
};

// Backward compatibility alias
window.showNotification = function (message, type = 'info') {
    window.showToast(message, type, 4000);
};

// ═════════════════════════════════════════════════════════════════════════
// SKELETON LOADERS
// ═════════════════════════════════════════════════════════════════════════

window.createSkeletonRows = function (count = 5) {
    return Array(count)
        .fill(0)
        .map(
            () => `
        <tr>
            <td class="px-3 py-2"><div class="skeleton-loader h-4 w-16 rounded"></div></td>
            <td class="px-3 py-2"><div class="skeleton-loader h-4 w-24 rounded"></div></td>
            <td class="px-3 py-2"><div class="skeleton-loader h-4 w-20 rounded"></div></td>
            <td class="px-3 py-2"><div class="skeleton-loader h-4 w-16 rounded"></div></td>
            <td class="px-3 py-2"><div class="skeleton-loader h-6 w-32 rounded"></div></td>
            <td class="px-3 py-2"><div class="skeleton-loader h-4 w-24 rounded"></div></td>
        </tr>
    `
        )
        .join('');
};

window.showTableSkeletons = function (tbodyId, count = 5) {
    const tbody = document.getElementById(tbodyId);
    if (tbody) {
        tbody.innerHTML = window.createSkeletonRows(count);
    }
};

// ═════════════════════════════════════════════════════════════════════════
// EMPTY STATES
// ═════════════════════════════════════════════════════════════════════════

const EMPTY_STATES = {
    tasks: {
        title: 'No Tasks Running',
        subtitle: 'Click "New Task" to begin reconnaissance.',
        icon: '🚀'
    },
    findings: {
        title: 'No Findings Yet',
        subtitle: 'Run a scan to discover vulnerabilities.',
        icon: '🔍'
    },
    reports: {
        title: 'No Reports Available',
        subtitle: 'Complete a task to generate a report.',
        icon: '📊'
    },
    campaigns: {
        title: 'No Campaigns Created',
        subtitle: 'Create a new phishing campaign to get started.',
        icon: '📧'
    },
    results: {
        title: 'No Results Yet',
        subtitle: 'Monitor your campaigns for real-time results.',
        icon: '📈'
    }
};

window.createEmptyState = function (stateKey = 'tasks', customTitle = null, customSubtitle = null, customIcon = null) {
    const state = EMPTY_STATES[stateKey] || EMPTY_STATES.tasks;
    const title = customTitle || state.title;
    const subtitle = customSubtitle || state.subtitle;
    const icon = customIcon || state.icon;

    return `
        <div class="flex flex-col items-center justify-center py-12 px-4">
            <div class="text-6xl mb-4 opacity-50">${icon}</div>
            <h3 class="text-lg font-semibold text-gray-300 mb-2">${title}</h3>
            <p class="text-sm text-gray-500 text-center max-w-xs">${subtitle}</p>
        </div>
    `;
};

// ═════════════════════════════════════════════════════════════════════════
// FORM VALIDATION WITH VISUAL FEEDBACK
// ═════════════════════════════════════════════════════════════════════════

window.validateCIDR = function (cidr) {
    const cidrRegex = /^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$/;
    if (!cidrRegex.test(cidr.trim())) return false;

    const parts = cidr.split('/');
    const octets = parts[0].split('.').map(Number);

    // Validate octets are 0-255
    if (octets.some((o) => isNaN(o) || o < 0 || o > 255)) return false;

    // Validate CIDR prefix length
    if (parts[1]) {
        const mask = Number(parts[1]);
        if (isNaN(mask) || mask < 0 || mask > 32) return false;
    }

    return true;
};

window.validateTarget = function (target) {
    target = target.trim();
    const cidrRegex = /^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$/;
    const domainRegex = /^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z0-9]([a-z0-9-]*[a-z0-9])?$/i;
    const ipRegex = /^(\d{1,3}\.){3}\d{1,3}$/;
    const urlRegex = /^https?:\/\/.+/i;

    return cidrRegex.test(target) || domainRegex.test(target) || ipRegex.test(target) || urlRegex.test(target);
};

window.validateEmail = function (email) {
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
};

window.setFieldError = function (fieldId, message) {
    const field = document.getElementById(fieldId);
    if (!field) return;

    field.classList.add('border-red-500', 'bg-red-950/20');
    field.classList.remove('border-vantage-border');

    let errorEl = field.parentElement?.querySelector('.field-error');
    if (!errorEl) {
        errorEl = document.createElement('div');
        errorEl.className = 'field-error text-xs text-red-400 mt-1 flex items-center gap-1';
        errorEl.innerHTML = '<svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z"/></svg><span></span>';
        field.parentElement.appendChild(errorEl);
    }
    errorEl.querySelector('span').textContent = message;
};

window.clearFieldError = function (fieldId) {
    const field = document.getElementById(fieldId);
    if (!field) return;

    field.classList.remove('border-red-500', 'bg-red-950/20');
    field.classList.add('border-vantage-border');

    const errorEl = field.parentElement?.querySelector('.field-error');
    if (errorEl) errorEl.remove();
};

window.setFieldSuccess = function (fieldId) {
    const field = document.getElementById(fieldId);
    if (!field) return;

    field.classList.add('border-emerald-600', 'bg-emerald-950/20');
    field.classList.remove('border-vantage-border', 'border-red-500', 'bg-red-950/20');

    // Remove error element if present
    field.parentElement?.querySelector('.field-error')?.remove();
};

// Real-time validation listeners
document.addEventListener('DOMContentLoaded', function () {
    // Auto-validate target fields on input
    document.addEventListener('input', function (e) {
        if (e.target.id === 'task-target') {
            const isValid = window.validateTarget(e.target.value);
            if (e.target.value.trim()) {
                if (isValid) {
                    window.setFieldSuccess('task-target');
                } else {
                    window.setFieldError('task-target', 'Invalid target: Use CIDR, IP, domain, or URL');
                }
            } else {
                window.clearFieldError('task-target');
            }
        }

        if (e.target.id === 'route-cidr') {
            const isValid = window.validateCIDR(e.target.value);
            if (e.target.value.trim()) {
                if (isValid) {
                    window.setFieldSuccess('route-cidr');
                } else {
                    window.setFieldError('route-cidr', 'Invalid CIDR block (e.g., 192.168.1.0/24)');
                }
            } else {
                window.clearFieldError('route-cidr');
            }
        }
    });
});

// ═════════════════════════════════════════════════════════════════════════
// RESPONSIVE TABLE UTILITIES
// ═════════════════════════════════════════════════════════════════════════

window.makeTableResponsive = function (tableId) {
    const table = document.getElementById(tableId);
    if (!table) return;

    // Add horizontal scroll wrapper if on small screen
    if (window.innerWidth < 1024) {
        const wrapper = document.createElement('div');
        wrapper.className = 'overflow-x-auto max-h-96 scroll-smooth';
        table.parentNode.insertBefore(wrapper, table);
        wrapper.appendChild(table);
    }
};

// ═════════════════════════════════════════════════════════════════════════
// BUTTON LOADING STATES
// ═════════════════════════════════════════════════════════════════════════

window.setButtonLoading = function (buttonId, isLoading = true) {
    const btn = document.getElementById(buttonId);
    if (!btn) return;

    if (isLoading) {
        btn.disabled = true;
        btn.innerHTML =
            '<svg class="animate-spin h-4 w-4 inline mr-2" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>Loading...';
    } else {
        btn.disabled = false;
        btn.innerHTML = btn.getAttribute('data-original-html') || 'Submit';
    }
};

// Store original button HTML
document.addEventListener('DOMContentLoaded', function () {
    document.querySelectorAll('button').forEach((btn) => {
        if (!btn.hasAttribute('data-original-html')) {
            btn.setAttribute('data-original-html', btn.innerHTML);
        }
    });
});

console.log('[UI Enhancements] All components loaded successfully');
