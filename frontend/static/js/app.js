document.addEventListener('alpine:init', () => {
    Alpine.data('ledgerApp', () => ({
        theme: localStorage.getItem('theme') || 'dark',
        token: '',
        currentView: 'dashboard',
        logoCacheBuster: Date.now(),
        showShutdown: false,
        verifications: [],
        searchQuery: '',
        verifyStatus: 'Checking...',
        verifyOk: true,
        toast: null,
        fiscalYears: [],
        activeFiscalYearId: null,
        formDateError: '',
        showInspector: false,
        inspector: { url: '', mime: '' },
        showSettings: false,
        showKontoplan: false,
        showMoms: false,
        showInvoices: false,
        showTools: false,
        showReports: false,
        showHelp: false,
        isLoadingReport: false,
        financialReport: null,
        invoices: [],
        selectedInvoice: null,
        customers: [],
        customersStale: true,
        showSealModal: false,
        showRestoreModal: false,
        restoreComplete: false,
        showBackupModal: false,
        backupPassword: '',
        isExportingBackup: false,
        restorePasswordRequired: false,
        restorePassword: '',
        restoreFileToUpload: null,
        isRestoring: false,
        showBugLogger: false,
        isSealing: false,
        bugLog: [],
        armedActionId: null,
        invoiceAbortController: null,
        showTips: true,
        showEulaModal: false,
        isEulaReadOnly: false,
        eulaAcceptedVersion: localStorage.getItem('eula_accepted_version') || '',
        currentEulaVersion: '1.0-beta',
        helpCheckbox1: localStorage.getItem('helpCheckbox1') === 'true',
        helpCheckbox2: localStorage.getItem('helpCheckbox2') === 'true',
        helpCheckbox3: localStorage.getItem('helpCheckbox3') === 'true',
        helpCheckbox4: localStorage.getItem('helpCheckbox4') === 'true',
        helpCheckbox5: localStorage.getItem('helpCheckbox5') === 'true',

        confirmModal: { 
            show: false, 
            title: '', 
            message: '', 
            confirmLabel: 'Bekräfta',
            isDangerous: false,
            resolve: null 
        },

        confirmAction(message, title = 'Bekräfta', confirmLabel = 'Bekräfta', isDangerous = false) {
            return new Promise((resolve) => {
                if (this.confirmModal.resolve) {
                    this.confirmModal.resolve(false);
                }

                this.confirmModal.title = title;
                this.confirmModal.message = message;
                this.confirmModal.confirmLabel = confirmLabel;
                this.confirmModal.isDangerous = isDangerous;
                this.confirmModal.resolve = resolve;
                this.confirmModal.show = true;

                this.$nextTick(() => {
                    if (this.$refs.confirmBtn) this.$refs.confirmBtn.focus();
                });
            });
        },
        
        showDashboard: true,
        isScanningOcr: false,
        ocrStatus: '',
        dashboard: { income: 0, expenses: 0, net: 0, bank: 0, outstanding_receivables: 0, unpaid_count: 0 },
        vatReport: null,
        vatStartDate: '',
        vatEndDate: '',
        vatPeriodPreset: '',
        basAccounts: [],
        settings: { name: '', org_number: '', cloud_inbox_path: '', logo_path: '' },
        newAccount: { code: '', name: '', type: 'Kostnad' },

        inboxItems: [],
        isInboxDrawerOpen: false,

        async fetchInbox() {
            try {
                const res = await this.authFetch('/api/inbox');
                if (res.ok) {
                    this.inboxItems = await res.json() || [];
                }
            } catch (e) { console.error("Failed to fetch inbox", e); }
        },

        async fetchCloudInbox() {
            this.showToast('Hämtar kvitton från molnet...', 'info');
            try {
                const res = await this.authFetch('/api/inbox/fetch-cloud', { method: 'POST' });
                if (res.ok) {
                    const data = await res.json();
                    if (data.fetched > 0) {
                        this.showToast(`Hämtade ${data.fetched} nya filer!`, 'success');
                        this.fetchInbox();
                    } else if (data.failed === 0) {
                        this.showToast('Inga nya filer i molnet.', 'info');
                    }

                    if (data.failed > 0) {
                        this.showToast(`${data.failed} filer misslyckades.`, 'error');
                        data.errors.forEach(err => {
                            this.showToast(err, 'error');
                        });
                    }
                } else {
                    const err = await res.json();
                    this.showToast('Fel: ' + err.error, 'error');
                }
            } catch (e) {
                this.showToast('Nätverksfel vid hämtning', 'error');
            }
        },

        async deleteInboxItem(id) {
            try {
                const res = await this.authFetch(`/api/inbox/${id}`, { method: 'DELETE' });
                if (res.ok) {
                    this.inboxItems = this.inboxItems.filter(i => i.id !== id);
                    this.showToast('Kvitto borttaget', 'success');
                } else {
                    this.showToast('Kunde inte ta bort kvitto', 'error');
                }
            } catch(e) {
                this.showToast('Nätverksfel', 'error');
            }
        },

        templates: [
            {
                id: 'inkop_25',
                name: 'Inköp utrustning (25% moms)',
                desc: 'Inköp förbrukningsinventarier',
                type: 'expense',
                vatRate: 0.25,
                accountTotal: '1930', // Kredit
                accountVat: '2641',   // Debet
                accountBase: '5410'   // Debet
            },
            {
                id: 'forsaljning_25',
                name: 'Försäljning tjänst (25% moms)',
                desc: 'Försäljning av tjänst inom Sverige',
                type: 'income',
                vatRate: 0.25,
                accountTotal: '1930', // Debet
                accountVat: '2611',   // Kredit
                accountBase: '3010'   // Kredit
            },
            {
                id: 'egen_insattning',
                name: 'Egen insättning (EF)',
                desc: 'Egen insättning',
                type: 'transfer',
                vatRate: 0.00,
                accountTotal: '1930', // Debet
                accountBase: '2018'   // Kredit
            },
            {
                id: 'eget_uttag',
                name: 'Eget uttag (EF)',
                desc: 'Eget uttag',
                type: 'transfer',
                vatRate: 0.00,
                accountTotal: '1930', // Kredit
                accountBase: '2013'   // Debet
            }
        ],
        selectedTemplate: '',
        templatePrompt: { active: false, templateId: '', amount: '' },

        initiateTemplate() {
            if (!this.selectedTemplate) return;
            this.templatePrompt.templateId = this.selectedTemplate;
            this.templatePrompt.active = true;
            this.selectedTemplate = ''; // Återställ dropdown direkt
            this.$nextTick(() => {
                if(this.$refs.templateAmountInput) {
                    this.$refs.templateAmountInput.focus();
                }
            });
        },

        cancelTemplate() {
            this.templatePrompt.active = false;
            this.templatePrompt.templateId = '';
            this.templatePrompt.amount = '';
        },

        applyTemplateAmount() {
            if (!this.templatePrompt.amount) {
                this.cancelTemplate();
                return;
            }
            
            const t = this.templates.find(x => x.id === this.templatePrompt.templateId);
            if (!t) return;
            
            const cleaned = this.templatePrompt.amount.toString().replace(',', '.').replace(/ /g, '');
            const amountFloat = parseFloat(cleaned);
            if (isNaN(amountFloat) || amountFloat <= 0) {
                this.showToast("Ogiltigt belopp", "error");
                return;
            }
            
            const totalOren = Math.round(amountFloat * 100);
            
            let vatOren = 0;
            let baseOren = totalOren;
            
            if (t.vatRate > 0) {
                const vatFraction = t.vatRate / (1 + t.vatRate);
                vatOren = Math.round(totalOren * vatFraction);
                baseOren = totalOren - vatOren;
            }
            
            if (!this.form.text) this.form.text = t.desc;
            this.form.rows = []; 
            
            if (t.type === 'expense') {
                this.form.rows.push({ _key: Date.now() + 1, account: t.accountTotal, debet: 0, kredit: totalOren / 100 });
                if (vatOren > 0) this.form.rows.push({ _key: Date.now() + 2, account: t.accountVat, debet: vatOren / 100, kredit: 0 });
                this.form.rows.push({ _key: Date.now() + 3, account: t.accountBase, debet: baseOren / 100, kredit: 0 });
            } else if (t.type === 'income') {
                this.form.rows.push({ _key: Date.now() + 1, account: t.accountTotal, debet: totalOren / 100, kredit: 0 });
                if (vatOren > 0) this.form.rows.push({ _key: Date.now() + 2, account: t.accountVat, debet: 0, kredit: vatOren / 100 });
                this.form.rows.push({ _key: Date.now() + 3, account: t.accountBase, debet: 0, kredit: baseOren / 100 });
            } else if (t.type === 'transfer' && t.id === 'egen_insattning') {
                this.form.rows.push({ _key: Date.now() + 1, account: t.accountTotal, debet: totalOren / 100, kredit: 0 });
                this.form.rows.push({ _key: Date.now() + 2, account: t.accountBase, debet: 0, kredit: totalOren / 100 });
            } else if (t.type === 'transfer' && t.id === 'eget_uttag') {
                this.form.rows.push({ _key: Date.now() + 1, account: t.accountTotal, debet: 0, kredit: totalOren / 100 });
                this.form.rows.push({ _key: Date.now() + 2, account: t.accountBase, debet: totalOren / 100, kredit: 0 });
            }
            
            this.cancelTemplate();
            this.showToast("Mall tillämpad", "success");
            
            this.$nextTick(() => {
                if (this.$refs.formDesc) {
                    this.$refs.formDesc.focus();
                    if (typeof this.$refs.formDesc.select === 'function') this.$refs.formDesc.select();
                }
            });
        },
        
        form: {
            date: new Date().toISOString().split('T')[0],
            text: '',
            attachmentBase64: '',
            attachmentName: '',
            attachmentMime: '',
            attachmentSize: '',
            attachmentUrl: '',
            rows: [
                { _key: Date.now() + 1, account: '1930', debet: 0, kredit: 0 },
                { _key: Date.now() + 2, account: '3010', debet: 0, kredit: 0 }
            ]
        },

        get activeFiscalYear() {
            return this.fiscalYears.find(f => f.id == this.activeFiscalYearId);
        },



        syncFormDateWithFiscalYear() {
            this.formDateError = '';
            const fy = this.activeFiscalYear;
            if (!fy) return;

            const current = this.form.date;
            if (current < fy.start_date || current > fy.end_date) {
                const today = new Date().toISOString().split('T')[0];
                if (today >= fy.start_date && today <= fy.end_date) {
                    this.form.date = today;
                } else {
                    this.form.date = fy.start_date;
                }
            }
        },

        async init() {
            this._toastTimeout = null;
            // Sentinel Principle: Bind early errors and set up persistence
            const stored = (() => { try { return JSON.parse(sessionStorage.getItem('bugLog')) || []; } catch { return []; } })();
            const early = (window.__earlyErrors || []).map(err => typeof err === 'object' ? `[${new Date(err.ts).toLocaleTimeString()}] EARLY ERROR: ${err.msg} at ${err.src}:${err.line}` : err);
            
            // Sentinel Principle: Registrera watch innan state uppdateras
            this.$watch('bugLog', val => sessionStorage.setItem('bugLog', JSON.stringify(val)));
            
            // Watch activeFiscalYearId to keep dashboard, verifications, and dates in sync
            this.$watch('activeFiscalYearId', val => {
                if (val) {
                    this.fetchDashboardData();
                    this.fetchVerifications();
                    this.syncFormDateWithFiscalYear();
                }
            });
            
            // Uppdateringen triggar nu sparandet direkt
            this.bugLog = [...stored, ...early];
            
            window.onerror = (message, source, lineno, colno, error) => {
                this.bugLog.push(`[${new Date().toLocaleTimeString()}] ERROR: ${message} at ${source}:${lineno}`);
            };
            window.onunhandledrejection = (event) => {
                this.bugLog.push(`[${new Date().toLocaleTimeString()}] PROMISE REJECTION: ${event.reason}`);
            };

            // Read token from DOM injection
            const meta = document.querySelector('meta[name="api-token"]');
            if (meta) {
                this.token = meta.getAttribute('content');
            } else {
                this.showToast('Authentication token missing!', 'error');
            }

            if (this.eulaAcceptedVersion !== this.currentEulaVersion) {
                this.showEulaModal = true;
                this.isEulaReadOnly = false;
            } else {
                await this.completeInit();
            }

            // Command Palette SPA Routing Hook
            setTimeout(() => {
                const startupView = sessionStorage.getItem('startup_view');
                const startupAction = sessionStorage.getItem('startup_action');
                
                if (startupView) {
                    sessionStorage.removeItem('startup_view');
                    window.dispatchEvent(new CustomEvent('cmd-nav', { detail: startupView }));
                } else if (startupAction) {
                    sessionStorage.removeItem('startup_action');
                    window.dispatchEvent(new CustomEvent('cmd-action', { detail: startupAction }));
                }
            }, 100);

            // Toggle dashboard med Alt+D
            window.addEventListener('keydown', (e) => {
                if (e.altKey && e.key.toLowerCase() === 'd') {
                    e.preventDefault();
                    this.showDashboard = !this.showDashboard;
                }
            });
        },

        async loadInvoices() {
            try {
                this.invoices = [];
                const res = await this.authFetch('/api/invoices');
                if (res.ok) {
                    this.invoices = await res.json();
                } else {
                    this.showToast("Kunde inte ladda fakturor", "error");
                }
            } catch(e) {
                console.error(e);
                this.showToast("Kunde inte ladda fakturor", "error");
            }
        },

        async fetchCustomers() {
            try {
                const res = await this.authFetch('/api/customers');
                if (res.ok) {
                    this.customers = await res.json();
                } else {
                    console.error("Failed to fetch customers", await res.text());
                }
            } catch (e) {
                console.error("Network error fetching customers", e);
            }
        },

        async anonymizeCustomer(id) {
            if (!await this.confirmAction("Vill du anonymisera den här kunden? Detta raderar personuppgifter enligt GDPR men behåller fakturahistoriken intakt.", "Anonymisera", "Ja, anonymisera", true)) return;
            try {
                const res = await this.authFetch(`/api/customers/${id}`, { method: 'DELETE' });
                if (res.ok) {
                    this.showToast("Kunden har anonymiserats", "success");
                    this.customersStale = true;
                    await this.fetchCustomers();
                    await this.loadInvoices();
                } else {
                    const text = await res.text();
                    console.error("Failed to anonymize customer", text);
                    this.showToast("Kunde inte anonymisera kund: " + text, "error");
                }
            } catch (e) {
                console.error("Network error anonymizing customer", e);
                this.showToast("Nätverksfel vid anonymisering", "error");
            }
        },

        async selectInvoice(inv) {
            if (this.invoiceAbortController) {
                this.invoiceAbortController.abort();
            }
            this.invoiceAbortController = new AbortController();

            try {
                const res = await this.authFetch(`/api/invoices/${inv.id}`, {
                    signal: this.invoiceAbortController.signal
                });
                if (!res.ok) throw new Error("Kunde inte hämta fakturadetaljer");
                const fullInv = await res.json();
                
                if (fullInv.status === 'utkast') {
                    this.selectedInvoice = JSON.parse(JSON.stringify(fullInv));
                    if (!this.selectedInvoice.items) this.selectedInvoice.items = [];
                    this.selectedInvoice.items.forEach(item => {
                        item.quantity_float = item.quantity / 100;
                        item.price_float = item.price_ex_vat / 100;
                    });
                } else {
                    this.selectedInvoice = fullInv;
                }
            } catch(e) {
                if (e.name === 'AbortError') return;
                console.error(e);
                this.showToast("Ett fel uppstod vid laddning av fakturan.", "error");
            }
        },

        createNewInvoice() {
            const today = new Date().toISOString().split('T')[0];
            let dueDate = new Date();
            dueDate.setDate(dueDate.getDate() + 30);

            this.selectedInvoice = {
                id: 0,
                customer_name: '',
                customer_orgnr: '',
                customer_address: '',
                date: today,
                due_date: dueDate.toISOString().split('T')[0],
                payment_terms_days: 30,
                status: 'utkast',
                items: [{ description: '', quantity_float: 1, price_float: 0, vat_rate: 25 }]
            };
            this.recalcInvoice();
        },

        updateInvoiceDueDate() {
            if(!this.selectedInvoice || !this.selectedInvoice.date) return;
            let d = new Date(this.selectedInvoice.date);
            d.setDate(d.getDate() + (this.selectedInvoice.payment_terms_days || 0));
            this.selectedInvoice.due_date = d.toISOString().split('T')[0];
        },

        addInvoiceItem() {
            if(!this.selectedInvoice) return;
            this.selectedInvoice.items.push({ description: '', quantity_float: 1, price_float: 0, vat_rate: 25 });
            this.recalcInvoice();
        },

        removeInvoiceItem(idx) {
            if(!this.selectedInvoice) return;
            this.selectedInvoice.items.splice(idx, 1);
            this.recalcInvoice();
        },

        recalcInvoice() {
            if(!this.selectedInvoice) return;
            let totalExVat = 0;
            let totalVat = 0;
            (this.selectedInvoice.items || []).forEach(item => {
                const qty = Math.round((item.quantity_float || 0) * 100);
                const price = Math.round((item.price_float || 0) * 100);
                item.quantity = qty;
                item.price_ex_vat = price;

                const lineExVat = Math.round((price * qty) / 100);
                const lineVat = Math.round((lineExVat * item.vat_rate) / 100);
                
                totalExVat += lineExVat;
                totalVat += lineVat;
            });
            this.selectedInvoice.total_amount = totalExVat + totalVat;
            this.selectedInvoice.total_vat = totalVat;
        },

        async saveInvoice(options = { silent: false }) {
            const silent = options && options.silent === true;
            if (!this.selectedInvoice.customer_name) {
                if (!silent) this.showToast('Kundnamn saknas', 'error');
                return false;
            }
            this.recalcInvoice();
            try {
                if (!this.selectedInvoice.date || !this.selectedInvoice.due_date) {
                    if (!silent) this.showToast("Datum och förfallodatum är obligatoriska", "error");
                    return false;
                }
                
                const fy = this.activeFiscalYear;
                if (fy && (this.selectedInvoice.date < fy.start_date || this.selectedInvoice.date > fy.end_date)) {
                    if (!silent) this.showToast("Fakturadatum ligger utanför det aktiva räkenskapsåret", "error");
                    return false;
                }

                const isNew = !this.selectedInvoice.id;
                const method = isNew ? 'POST' : 'PUT';
                const url = isNew ? '/api/invoices' : `/api/invoices/${this.selectedInvoice.id}`;

                const res = await this.authFetch(url, {
                    method,
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(this.selectedInvoice)
                });
                
                if (res.ok) {
                    if (isNew) {
                        const data = await res.json();
                        this.selectedInvoice.id = data.id;
                    }
                    this.customersStale = true;
                    if (!silent) {
                        this.showToast("Fakturan sparades", "success");
                        await this.loadInvoices();
                        const updated = this.invoices.find(i => i.id === this.selectedInvoice.id);
                        if(updated) this.selectInvoice(updated);
                    }
                    return true;
                } else {
                    const errText = await res.text();
                    if (!silent) this.showToast("Kunde inte spara fakturan: " + errText, "error");
                    return false;
                }
            } catch(e) {
                console.error(e);
                if (!silent) this.showToast("Nätverksfel vid sparande", "error");
                return false;
            }
        },

        async postInvoice() {
            if (!this.selectedInvoice) return;
            const proceed = await this.confirmAction("Vill du bokföra fakturan? Detta går inte att ångra och fakturan får sitt officiella fakturanummer.", "Lås Faktura", "Bokför", true);
            if (!proceed) return;

            // Auto-save draft before posting to capture all unsaved changes without flashing toasts or swapping refs.
            const saved = await this.saveInvoice({ silent: true });
            if (!saved) return;
            
            try {
                const res = await this.authFetch(`/api/invoices/${this.selectedInvoice.id}/post`, { method: 'POST' });
                if (res.ok) {
                    this.showToast("Fakturan bokfördes!", "success");
                    await this.loadInvoices();
                    const updated = this.invoices.find(i => i.id === this.selectedInvoice.id);
                    if(updated) this.selectInvoice(updated);
                } else {
                    const errText = await res.text();
                    this.showToast("Fel vid bokföring: " + errText, "error");
                }
            } catch(e) {
                console.error(e);
            }
        },

        async payInvoice() {
            const dateStr = prompt("Ange datum för inbetalningen (YYYY-MM-DD)", new Date().toISOString().split('T')[0]);
            if (!dateStr) return;

            const d = new Date(dateStr);
            if (isNaN(d.getTime()) || d.toISOString().slice(0, 10) !== dateStr) {
                this.showToast("Ogiltigt datumformat.", 'error');
                return;
            }

            try {
                const res = await this.authFetch(`/api/invoices/${this.selectedInvoice.id}/pay`, {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({ date: dateStr })
                });
                if (res.ok) {
                    this.showToast("Betalning registrerad", "success");
                    await this.loadInvoices();
                    const updated = this.invoices.find(i => i.id === this.selectedInvoice.id);
                    if(updated) this.selectInvoice(updated);
                } else {
                    const errText = await res.text();
                    this.showToast("Fel vid inbetalning: " + errText, "error");
                }
            } catch(e) {
                console.error(e);
            }
        },

        async creditInvoice(id) {
            try {
                const res = await this.authFetch(`/api/invoices/${id}/credit`, {
                    method: 'POST'
                });
                if (res.ok) {
                    const data = await res.json();
                    this.showToast("Kreditfaktura skapad!", "success");
                    await this.loadInvoices();
                    const updated = this.invoices.find(i => i.id === data.id);
                    if (updated) {
                        this.selectInvoice(updated);
                    }
                } else {
                    const errText = await res.text();
                    this.showToast("Fel vid skapande av kreditfaktura: " + errText, "error");
                }
            } catch (e) {
                console.error(e);
            }
        },

        async downloadInvoicePDF(id) {
            try {
                this.showToast("Genererar PDF...", "info");
                const res = await this.authFetch(`/api/invoices/${id}/pdf`);
                if (res.ok) {
                    const blob = await res.blob();
                    const urlBlob = window.URL.createObjectURL(blob);
                    const a = document.createElement('a');
                    a.href = urlBlob;
                    a.download = `Faktura_${id}.pdf`;
                    document.body.appendChild(a);
                    a.click();
                    window.URL.revokeObjectURL(urlBlob);
                    a.remove();
                    this.showToast("PDF nedladdad!", "success");
                } else {
                    const errText = await res.text();
                    this.showToast("Kunde inte ladda ner PDF: " + errText, "error");
                }
            } catch(e) {
                console.error(e);
                this.showToast("Nätverksfel vid PDF-nedladdning", "error");
            }
        },

        async printInvoicePDF(id) {
            if (this._printingInProgress) {
                this.showToast("En utskrift pågår redan. Vänligen vänta.", "warning");
                return;
            }
            try {
                this._printingInProgress = true;
                this.showToast("Förbereder utskrift...", "info");
                const res = await this.authFetch(`/api/invoices/${id}/pdf`);
                if (res.ok) {
                    const blob = await res.blob();
                    const url = URL.createObjectURL(blob);
                    
                    let iframe = document.getElementById('print-iframe');
                    if (!iframe) {
                        iframe = document.createElement('iframe');
                        iframe.id = 'print-iframe';
                        iframe.style.position = 'fixed';
                        iframe.style.width = '0';
                        iframe.style.height = '0';
                        iframe.style.border = 'none';
                        document.body.appendChild(iframe);
                    }
                    iframe.src = url;
                    iframe.onload = () => {
                        iframe.contentWindow.focus();
                        iframe.contentWindow.print();
                        
                        const cleanup = () => {
                            try {
                                URL.revokeObjectURL(url);
                            } catch(e){}
                            this._printingInProgress = false;
                        };
                        
                        iframe.contentWindow.addEventListener('afterprint', cleanup, { once: true });
                        setTimeout(cleanup, 300000); // 5 minute fallback
                    };
                } else {
                    this._printingInProgress = false;
                    const errText = await res.text();
                    this.showToast("Kunde inte hämta PDF för utskrift: " + errText, "error");
                }
            } catch (e) {
                this._printingInProgress = false;
                console.error(e);
                this.showToast("Utskrift misslyckades: " + e.message, "error");
            }
        },

        async settleInvoice(id) {
            const proceed = await this.confirmAction("Vill du kvitta denna kreditfaktura mot originalfakturan?", "Kvitta", "Kvitta", false);
            if (!proceed) return;

            try {
                const res = await this.authFetch(`/api/invoices/${id}/settle`, {
                    method: 'POST'
                });
                if (res.ok) {
                    this.showToast("Kvittning utförd!", "success");
                    await this.loadInvoices();
                    const updated = this.invoices.find(i => i.id === id);
                    if (updated) {
                        this.selectInvoice(updated);
                    }
                } else {
                    const errText = await res.text();
                    this.showToast("Fel vid kvittning: " + errText, "error");
                }
            } catch (e) {
                console.error(e);
            }
        },

        async deleteInvoice() {
            if (!this.selectedInvoice) return;
            if (!this.selectedInvoice.id || this.selectedInvoice.id === 0) {
                this.selectedInvoice = null;
                return;
            }
            const proceed = await this.confirmAction("Vill du radera detta utkast?", "Radera", "Radera", true);
            if (!proceed) return;
            try {
                const res = await this.authFetch(`/api/invoices/${this.selectedInvoice.id}`, { method: 'DELETE' });
                if (res.ok) {
                    this.selectedInvoice = null;
                    await this.loadInvoices();
                    this.showToast("Utkastet raderades", "success");
                } else {
                    this.showToast("Kunde inte radera utkastet", "error");
                }
            } catch(e) {
                console.error(e);
            }
        },

        formatMoneyFloat(val) {
            if (val === undefined || val === null) return "0.00 kr";
            return val.toFixed(2) + " kr";
        },

        async loadFinancialReport() {
            this.isLoadingReport = true;
            try {
                const url = this.activeFiscalYearId ? `/api/reports/financial?year_id=${this.activeFiscalYearId}` : '/api/reports/financial';
                const res = await this.authFetch(url);
                if (res.ok) {
                    this.financialReport = await res.json();
                } else {
                    this.showToast("Kunde inte ladda finansiell rapport", "error");
                }
            } catch (e) {
                console.error(e);
                this.showToast("Kunde inte ladda finansiell rapport", "error");
            } finally {
                this.isLoadingReport = false;
            }
        },

        async downloadSamlingsplan() {
            try {
                const url = this.activeFiscalYearId ? `/api/reports/samlingsplan?year_id=${this.activeFiscalYearId}` : '/api/reports/samlingsplan';
                const res = await this.authFetch(url);
                if (res.ok) {
                    const blob = await res.blob();
                    const urlBlob = window.URL.createObjectURL(blob);
                    const a = document.createElement('a');
                    a.href = urlBlob;
                    a.download = `Samlingsplan_${this.activeFiscalYearId || 'all'}.csv`;
                    document.body.appendChild(a);
                    a.click();
                    window.URL.revokeObjectURL(urlBlob);
                    a.remove();
                } else {
                    this.showToast("Kunde inte ladda ner samlingsplan", "error");
                }
            } catch (e) {
                console.error(e);
                this.showToast("Kunde inte ladda ner samlingsplan", "error");
            }
        },

        downloadExcel() {
            try {
                if(!this.financialReport) return;
                const data = this.financialReport;
                const sep = "\t"; // Excel gillar tabb för CSV i svensk locale
                
                const fmt = (val) => {
                    if(!val) return "0,00";
                    return val.toFixed(2).replace('.', ',');
                };
                
                let csv = `Finansiell Rapport - ${data.FiscalYear}\n\n`;
                
                csv += `Intäkter${sep}${sep}\n`;
                csv += `Konto${sep}Beskrivning${sep}Belopp\n`;
                if(data.Income) data.Income.forEach(r => {
                    csv += `${r.AccountCode}${sep}${r.AccountName}${sep}${fmt(r.Balance/100)}\n`;
                });
                csv += `Summa Intäkter${sep}${sep}${fmt(data.TotalIncome/100)}\n\n`;
                
                csv += `Kostnader${sep}${sep}\n`;
                csv += `Konto${sep}Beskrivning${sep}Belopp\n`;
                if(data.Expenses) data.Expenses.forEach(r => {
                    csv += `${r.AccountCode}${sep}${r.AccountName}${sep}${fmt(r.Balance/100)}\n`;
                });
                csv += `Summa Kostnader${sep}${sep}${fmt(data.TotalExpenses/100)}\n\n`;
                
                csv += `Årets Resultat${sep}${sep}${fmt(data.NetIncome/100)}\n\n`;

                csv += `Tillgångar${sep}${sep}\n`;
                csv += `Konto${sep}Beskrivning${sep}Belopp\n`;
                if(data.Assets) data.Assets.forEach(r => {
                    csv += `${r.AccountCode}${sep}${r.AccountName}${sep}${fmt(r.Balance/100)}\n`;
                });
                csv += `Summa Tillgångar${sep}${sep}${fmt(data.TotalAssets/100)}\n\n`;

                csv += `Skulder och Eget Kapital${sep}${sep}\n`;
                csv += `Konto${sep}Beskrivning${sep}Belopp\n`;
                if(data.Liabilities) data.Liabilities.forEach(r => {
                    csv += `${r.AccountCode}${sep}${r.AccountName}${sep}${fmt(r.Balance/100)}\n`;
                });
                csv += `Summa Skulder${sep}${sep}${fmt(data.TotalLiabilities/100)}\n`;
                csv += `2099${sep}Årets Resultat${sep}${fmt(data.NetIncome/100)}\n`;
                csv += `Summa Skulder & Eget Kapital${sep}${sep}${fmt(data.CalculatedEquity/100)}\n`;

                const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
                const urlObj = URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = urlObj;
                a.download = `LocalLedger_Rapport_${data.FiscalYear}.csv`;
                document.body.appendChild(a);
                a.click();
                document.body.removeChild(a);
                URL.revokeObjectURL(urlObj);
            } catch (err) {
                console.error("Fel vid CSV-generering:", err);
                this.showToast("Kunde inte generera Excel-filen", "error");
            }
        },

        async fetchAccounts() {
            try {
                const res = await this.authFetch('/api/accounts');
                if (res.ok) {
                    this.basAccounts = await res.json();
                } else {
                    console.error("Failed to fetch accounts", await res.text());
                    this.showToast("Kunde inte ladda kontoplanen från servern", "error");
                }
            } catch (e) {
                console.error("Network error fetching accounts", e);
                this.showToast("Nätverksfel: Kunde inte ladda kontoplanen", "error");
            }
        },

        async shutdownServer() {
            if (!await this.confirmAction("Vill du stänga av LocalLedger?", "Stäng av", "Stäng av", true)) return;
            try {
                if (window.pingInterval) {
                    clearInterval(window.pingInterval);
                }
                await this.authFetch('/api/shutdown', { method: 'POST' });
                this.showShutdown = true;
            } catch (e) {
                console.error("Failed to shutdown", e);
                this.showShutdown = true; // Ensure glassmorphic overlay is shown even if fetch fails due to sudden server death
            }
        },

        async fetchFiscalYears() {
            try {
                const res = await this.authFetch('/api/fiscal-years');
                if (res.ok) {
                    const data = await res.json();
                    this.fiscalYears = data || [];
                    if (this.fiscalYears.length > 0 && !this.activeFiscalYearId) {
                        this.activeFiscalYearId = this.fiscalYears[0].id;
                    }
                }
            } catch (e) {
                console.error("Failed to fetch fiscal years", e);
            }
        },

        async createFiscalYear() {
            if (!await this.confirmAction("Är du säker på att du vill stänga detta år och föra över balanser till ett nytt räkenskapsår?", "Stäng år", "Stäng & Skapa nytt")) return;
            try {
                const res = await this.authFetch('/api/fiscal-years', { method: 'POST' });
                if (res.ok) {
                    this.showToast('Nytt räkenskapsår skapat framgångsrikt', 'success');
                    await this.fetchFiscalYears();
                    this.activeFiscalYearId = this.fiscalYears[0].id; // Välj det nya
                    this.syncFormDateWithFiscalYear();
                    this.fetchVerifications();
                } else {
                    const err = await res.json();
                    this.showToast('Misslyckades: ' + err.error, 'error');
                }
            } catch (e) {
                this.showToast('Nätverksfel vid skapande av nytt år', 'error');
            }
        },

        async lockFiscalYear(id) {
            if (!await this.confirmAction("VARNING: Att låsa ett räkenskapsår är permanent. Inga fler ändringar eller makuleringar kan göras. Fortsätt?", "Lås Räkenskapsår", "Lås året", true)) return;
            try {
                const res = await this.authFetch(`/api/fiscal-years/${id}/lock`, { method: 'POST' });
                if (res.ok) {
                    this.showToast('Räkenskapsåret har WORM-förseglats och låsts', 'success');
                    await this.fetchFiscalYears();
                    this.runVerification();
                } else {
                    const err = await res.json();
                    this.showToast('Misslyckades låsa: ' + err.error, 'error');
                }
            } catch (e) {
                this.showToast('Nätverksfel', 'error');
            }
        },

        async authFetch(url, options = {}) {
            if (!options.headers) options.headers = {};
            options.headers['Authorization'] = 'Bearer ' + this.token;
            options.headers['Content-Type'] = 'application/json';
            return fetch(url, options);
        },

        async fetchVerifications() {
            try {
                let url = '/api/verifications';
                if (this.activeFiscalYearId) {
                    url += '?year_id=' + this.activeFiscalYearId;
                }
                const res = await this.authFetch(url);
                if (res.ok) {
                    const data = await res.json();
                    // Sortera fallande (nyaste först)
                    this.verifications = (data || []).sort((a, b) => b.id - a.id);
                } else {
                    const err = await res.json();
                    this.showToast('Misslyckades hämta data: ' + err.error, 'error');
                }
            } catch (e) {
                this.showToast('Nätverksfel vid hämtning av data', 'error');
            }
        },

        get filteredVerifications() {
            if (!this.searchQuery) return this.verifications;
            const q = this.searchQuery.toLowerCase();
            return this.verifications.filter(v => {
                const idMatch = v.id.toString().includes(q);
                const textMatch = v.text.toLowerCase().includes(q);
                const dateMatch = v.date.includes(q);
                const rowMatch = v.rows && v.rows.some(r => r.account.includes(q));
                return idMatch || textMatch || dateMatch || rowMatch;
            });
        },

        async runVerification() {
            try {
                const res = await this.authFetch('/api/verify');
                if (res.ok) {
                    this.verifyOk = true;
                    this.verifyStatus = 'WORM Chain Valid';
                } else {
                    const err = await res.json();
                    this.verifyOk = false;
                    this.verifyStatus = 'Integrity Violation!';
                    this.showToast(err.error, 'error');
                }
            } catch (e) {
                this.verifyOk = false;
                this.verifyStatus = 'Offline';
            }
        },

        async fetchSettings() {
            try {
                const res = await this.authFetch('/api/settings');
                if (res.ok) {
                    const data = await res.json();
                    if (data.name) this.settings = data;
                }
            } catch (e) { console.error(e); }
        },

        async saveSettings() {
            try {
                const res = await this.authFetch('/api/settings', {
                    method: 'POST',
                    body: JSON.stringify(this.settings)
                });
                if (res.ok) {
                    this.showToast('Inställningar sparade', 'success');
                } else {
                    this.showToast('Fel vid sparning', 'error');
                }
            } catch (e) {
                this.showToast('Nätverksfel', 'error');
            }
        },

        async createDesktopShortcut() {
            try {
                const res = await this.authFetch('/api/settings/shortcut', {
                    method: 'POST'
                });
                if (res.ok) {
                    this.showToast('Skrivbordsgenväg skapad!', 'success');
                } else {
                    const data = await res.json().catch(() => ({}));
                    this.showToast(data.error || 'Kunde inte skapa genväg', 'error');
                }
            } catch (e) {
                console.error(e);
                this.showToast('Nätverksfel vid skapande av genväg', 'error');
            }
        },

        async uploadLogo(event) {
            let file;
            if (event.dataTransfer && event.dataTransfer.files) {
                file = event.dataTransfer.files[0];
            } else if (event.target && event.target.files) {
                file = event.target.files[0];
            }
            if (!file) return;

            if (file.size > 5 * 1024 * 1024) {
                this.showToast('Filen är för stor (max 5MB)', 'error');
                return;
            }

            const ext = file.name.substring(file.name.lastIndexOf('.')).toLowerCase();
            if (ext !== '.svg' && ext !== '.png' && ext !== '.jpg' && ext !== '.jpeg') {
                this.showToast('Endast SVG, PNG och JPG/JPEG-filer är tillåtna', 'error');
                return;
            }

            const formData = new FormData();
            formData.append('logo_file', file);

            try {
                this.showToast('Laddar upp logotyp...', 'info');
                const res = await fetch('/api/settings/logo', {
                    method: 'POST',
                    headers: { 'Authorization': `Bearer ${this.token}` },
                    body: formData
                });

                if (res.ok) {
                    const data = await res.json();
                    this.settings.logo_path = data.logo_path;
                    this.logoCacheBuster = Date.now(); // force logo images to reload
                    this.showToast('Logotyp uppladdad!', 'success');
                } else {
                    const err = await res.json();
                    this.showToast('Kunde inte ladda upp logotyp: ' + err.error, 'error');
                }
            } catch (e) {
                console.error("Nätverksfel vid uppladdning av logotyp", e);
                this.showToast('Nätverksfel vid uppladdning av logotyp', 'error');
            }
        },

        guessAccountType() {
            if (!this.newAccount.code) return;
            const firstDigit = this.newAccount.code.charAt(0);
            if (firstDigit === '1') this.newAccount.type = 'Tillgång';
            else if (firstDigit === '2') this.newAccount.type = 'Skuld';
            else if (firstDigit === '3') this.newAccount.type = 'Intäkt';
            else if (firstDigit >= '4' && firstDigit <= '8') {
                if (this.newAccount.code.startsWith('83')) this.newAccount.type = 'Intäkt';
                else this.newAccount.type = 'Kostnad';
            }
        },

        async saveAccount() {
            try {
                const res = await this.authFetch('/api/accounts', {
                    method: 'POST',
                    body: JSON.stringify(this.newAccount)
                });
                if (res.ok) {
                    this.showToast('Konto sparat', 'success');
                    this.newAccount = { code: '', name: '', type: 'Kostnad' };
                    this.fetchAccounts(); // Ladda om kontoplanen
                } else {
                    const err = await res.json();
                    this.showToast('Fel: ' + err.error, 'error');
                }
            } catch (e) {
                this.showToast('Nätverksfel', 'error');
            }
        },

        async stornoVerification(id) {
            try {
                const res = await this.authFetch(`/api/verifications/${id}/storno`, { method: 'POST' });
                if (res.ok) {
                    this.showToast('Verifikation stornerad framgångsrikt', 'success');
                    this.fetchVerifications();
                    this.fetchDashboardData();
                } else {
                    const err = await res.json();
                    this.showToast('Fel vid stornering: ' + err.error, 'error');
                }
            } catch (e) {
                this.showToast('Nätverksfel vid stornering', 'error');
            }
        },

        async voidDraft(id) {
            try {
                const res = await this.authFetch(`/api/verifications/${id}/void`, { method: 'POST' });
                if (res.ok) {
                    this.showToast('Utkast makulerat', 'success');
                    this.fetchVerifications();
                    this.fetchDashboardData();
                } else {
                    const err = await res.json();
                    this.showToast('Fel vid makulering: ' + err.error, 'error');
                }
            } catch (e) {
                this.showToast('Nätverksfel', 'error');
            }
        },

        async fetchDashboardData() {
            if (!this.activeFiscalYearId) return;
            try {
                const res = await this.authFetch(`/api/dashboard?year_id=${this.activeFiscalYearId}`);
                if (!res.ok) return;
                const data = await res.json();
                this.dashboard.income = data.income || 0;
                this.dashboard.expenses = data.expenses || 0;
                this.dashboard.net = data.net_income || 0;
                this.dashboard.bank = data.bank_balance || 0;
                this.dashboard.outstanding_receivables = data.outstanding_receivables || 0;
                this.dashboard.unpaid_count = data.unpaid_count || 0;
            } catch (e) {
                console.error("Kunde inte hämta dashboard-data", e);
            }
        },

        async refreshAllData() {
            try {
                await Promise.all([
                    this.fetchVerifications(),
                    this.fetchAccounts(),
                    this.fetchDashboardData(),
                    this.fetchFiscalYears(),
                    this.fetchInbox(),
                    this.fetchCustomers()
                ]);
                
                // Om vi har ett aktivt momsdatumintervall, uppdatera rapporten också
                if (this.vatStartDate && this.vatEndDate) {
                    await this.calculateVatReport();
                }
            } catch (e) {
                console.error("Fel vid uppdatering av data: ", e);
                this.showToast("Misslyckades att uppdatera sidans data: " + e.message, "error");
            }
        },

        updateDashboardLocally(rows) {
            let addedIncome = 0;
            let addedExpense = 0;
            let addedAssets = 0;

            for (const row of rows) {
                const accStr = String(row.account);
                const debet = row.debet ? Math.round(row.debet * 100) : 0;
                const kredit = row.kredit ? Math.round(row.kredit * 100) : 0;
                
                // Baserat på standardiserad BAS-kontoplan
                if (accStr.startsWith('1')) {
                    addedAssets += (debet - kredit);
                } else if (accStr.startsWith('3')) {
                    addedIncome += (kredit - debet);
                } else if (/^[4-8]/.test(accStr)) {
                    addedExpense += (debet - kredit);
                }
            }

            this.dashboard.income += addedIncome;
            this.dashboard.expenses += addedExpense;
            this.dashboard.bank += addedAssets;
            this.dashboard.net = this.dashboard.income - this.dashboard.expenses;
        },

        async sealVerifications() {
            try {
                const res = await this.authFetch('/api/maintenance/seal', { method: 'POST' });
                if (res.ok) {
                    const data = await res.json();
                    if (data.Count > 0) {
                        this.showToast(`Utkast låsta: ${data.Count} verifikationer har förseglats i WORM-kedjan!`, 'success');
                    } else {
                        this.showToast('Inga utkast förseglades. Alla befintliga utkast har skapats under de senaste 24 timmarna.', 'warning');
                    }
                    this.fetchVerifications();
                    this.runVerification();
                } else {
                    const err = await res.json();
                    this.showToast('Fel vid låsning: ' + err.error, 'error');
                }
            } catch (e) {
                this.showToast('Nätverksfel', 'error');
            }
        },

        async calculateVatReport() {
            if (!this.vatStartDate || !this.vatEndDate) {
                this.showToast('Välj start och slutdatum', 'error');
                return;
            }
            try {
                const res = await fetch(`/api/vat-report?start_date=${this.vatStartDate}&end_date=${this.vatEndDate}`, {
                    headers: { 'Authorization': `Bearer ${this.token}` }
                });
                if (!res.ok) throw new Error((await res.json()).error);
                this.vatReport = await res.json();
            } catch (e) {
                this.showToast('Kunde inte beräkna moms: ' + e.message, 'error');
                this.vatReport = null;
            }
        },

        skv(ore) {
            if (!ore) return 0;
            return Math.round(ore / 100);
        },

        async transferVat() {
            if (!this.vatReport) return;
            if (this.vatReport.net_vat === 0) {
                this.showToast('Finns inget att omföra (Nettot är 0).', 'error');
                return;
            }
            if (!await this.confirmAction(`Är du säker? Detta skapar en verifikation som nollställer moms och därefter LÅSER alla månader i perioden (${this.vatStartDate} till ${this.vatEndDate}) för framtida ändringar.`, 'Nollställ Moms', 'Bokför Moms', true)) return;

            try {
                const res = await fetch('/api/vat-report/transfer', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${this.token}`
                    },
                    body: JSON.stringify({
                        start_date: this.vatStartDate,
                        end_date: this.vatEndDate
                    })
                });
                if (!res.ok) throw new Error((await res.json()).error);
                this.showToast('Momsen är nu omförd och perioden låst!', 'success');
                this.vatReport = null;
                await this.refreshAllData();
            } catch (e) {
                this.showToast('Fel vid momsomföring: ' + e.message, 'error');
            }
        },

        setVatPreset(preset) {
            this.vatPeriodPreset = preset;
            if (!preset) return;

            const fy = this.fiscalYears.find(f => f.id == this.activeFiscalYearId);
            if (!fy) return;

            const start = new Date(fy.start_date);
            let periodStart = new Date(start);
            let periodEnd = new Date(start);

            if (preset === 'Year') {
                periodEnd = new Date(fy.end_date);
            } else {
                let offsetMonths = 0;
                if (preset === 'Q1') offsetMonths = 0;
                else if (preset === 'Q2') offsetMonths = 3;
                else if (preset === 'Q3') offsetMonths = 6;
                else if (preset === 'Q4') offsetMonths = 9;

                periodStart.setMonth(start.getMonth() + offsetMonths);
                periodEnd = new Date(periodStart);
                periodEnd.setMonth(periodStart.getMonth() + 3);
                periodEnd.setDate(periodEnd.getDate() - 1);
            }

            this.vatStartDate = periodStart.toISOString().split('T')[0];
            this.vatEndDate = periodEnd.toISOString().split('T')[0];
            
            if (this.vatStartDate && this.vatEndDate) {
                this.calculateVatReport();
            }
        },

        get netVatText() {
            if (!this.vatReport) return '';
            return this.vatReport.net_vat > 0 ? 'Att Betala (Skuld till SKV)' : 'Få Tillbaka (Fordran)';
        },

        async fillGaps() {
            if (!await this.confirmAction('Vill du leta upp luckor i nummerserien och fylla dem med makuleringsposter (Enligt BFL)?', 'Laga Nummerserie', 'Laga')) return;
            try {
                const res = await this.authFetch('/api/maintenance/fill-gaps', { method: 'POST' });
                if (res.ok) {
                    const data = await res.json();
                    this.showToast(data.message, 'success');
                    this.fetchVerifications();
                } else {
                    const err = await res.json();
                    this.showToast('Fel vid lagning av nummerserie: ' + err.error, 'error');
                }
            } catch (e) {
                this.showToast('Nätverksfel', 'error');
            }
        },

        openBackupModal() {
            this.showBackupModal = true;
            this.backupPassword = '';
        },

        async exportBackup(password = '') {
            this.isExportingBackup = true;
            try {
                const url = '/api/export/backup';
                const res = await this.authFetch(url, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ password })
                });
                if (!res.ok) {
                    this.showToast('Misslyckades att exportera backup', 'error');
                    return;
                }
                const blob = await res.blob();
                const downloadUrl = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = downloadUrl;
                const timestamp = new Date().toISOString().replace(/[-:T]/g, '').slice(0, 14);
                a.download = password ? `LocalLedger_Backup_${timestamp}.zip.enc` : `LocalLedger_Backup_${timestamp}.zip`;
                document.body.appendChild(a);
                a.click();
                a.remove();
                window.URL.revokeObjectURL(downloadUrl);
                this.showToast('Säkerhetskopia hämtad!', 'success');
                this.showBackupModal = false;
                this.backupPassword = '';
            } catch (e) {
                this.showToast('Nätverksfel vid export', 'error');
            } finally {
                this.isExportingBackup = false;
            }
        },

        async uploadRestoreFile(event, password = '') {
            let file = this.restoreFileToUpload;
            if (event && event.target.files && event.target.files.length > 0) {
                file = event.target.files[0];
                this.restoreFileToUpload = file;
            }

            if (!file) return;

            const formData = new FormData();
            formData.append('backup_zip', file);
            if (password) {
                formData.append('password', password);
            }

            this.isRestoring = true;
            try {
                this.showToast('Återställer databasen. Vänligen vänta...', 'info');
                const res = await fetch('/api/import/backup', {
                    method: 'POST',
                    headers: { 'Authorization': `Bearer ${this.token}` },
                    body: formData
                });
                
                if (!res.ok) {
                    let errText = '';
                    try {
                        const jsonErr = await res.json();
                        if (jsonErr.error === 'password_required' || jsonErr.error === 'invalid_password') {
                            this.restorePasswordRequired = true;
                            this.showToast(jsonErr.error === 'invalid_password' ? 'Felaktigt lösenord!' : 'Lösenord krävs för denna krypterade backup.', 'error');
                            this.isRestoring = false;
                            return;
                        }
                        errText = jsonErr.error;
                    } catch(e) {
                        errText = await res.text();
                    }
                    throw new Error(errText || 'Okänt fel vid återställning');
                }
                
                this.restoreComplete = true;
                this.restorePasswordRequired = false;
                this.restoreFileToUpload = null;
                this.restorePassword = '';
                this.showToast('Återställning klar!', 'success');
            } catch(e) {
                this.showToast('Återställning misslyckades: ' + e.message, 'error');
            } finally {
                this.isRestoring = false;
                if (event && event.target) {
                    event.target.value = '';
                }
            }
        },

	        async uploadSIEFile(event) {
            const files = event.target.files;
            if (!files || files.length === 0) return;
            const file = files[0];

            if (!await this.confirmAction(`Är du säker på att du vill importera ${file.name} till det aktiva räkenskapsåret?`, 'Importera SIE', 'Importera', true)) {
                event.target.value = '';
                return;
            }

            const formData = new FormData();
            formData.append('file', file);
            formData.append('yearID', this.activeYearID);

            try {
                const res = await fetch('/api/import/sie4', {
                    method: 'POST',
                    headers: { 'Authorization': `Bearer ${this.token}` },
                    body: formData
                });
                
                if (!res.ok) {
                    const err = await res.json();
                    throw new Error(err.error);
                }
                
                this.showToast('SIE-fil importerad framgångsrikt!', 'success');
                await this.refreshAllData();
            } catch(e) {
                this.showToast('Import misslyckades: ' + e.message, 'error');
            }
            event.target.value = '';
        },

        async generateIB() {
            const fromYearStr = prompt("Ange ID för det föregående räkenskapsåret som ska stängas (t.ex. 1):");
            if (!fromYearStr) return;
            
            const fromYearID = parseInt(fromYearStr);
            if (isNaN(fromYearID)) return;

            if (!await this.confirmAction(`Ska vi överföra Utgående Balans från år ${fromYearID} som Ingående Balans till det valda aktiva året?`, 'Överför Balans', 'Överför')) return;

            try {
                const res = await this.authFetch(`/api/fiscal-years/${this.activeYearID}/generate-ib`, {
                    method: 'POST',
                    body: JSON.stringify({ from_year_id: fromYearID })
                });

                if (!res.ok) {
                    const err = await res.json();
                    throw new Error(err.error);
                }

                this.showToast('Ingående Balans skapad!', 'success');
                await this.refreshAllData();
            } catch(e) {
                this.showToast('Misslyckades att generera IB: ' + e.message, 'error');
            }
        },

        formatBytes(bytes, decimals = 2) {
            if (!+bytes) return '0 Bytes';
            const k = 1024;
            const dm = decimals < 0 ? 0 : decimals;
            const sizes = ['Bytes', 'KB', 'MB', 'GB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
        },

        async loadInboxItemToForm(item) {
            if (this.form.attachmentBase64 && !confirm('Ett kvitto är redan inläst. Vill du skriva över pågående verifikation?')) {
                return;
            }

            this.isInboxDrawerOpen = false; // Auto-close drawer
            this.showToast('Laddar in kvitto från inkorgen...', 'info');
            
            try {
                const res = await this.authFetch(`/api/inbox/${item.id}/download`);
                if (!res.ok) throw new Error("Kunde inte hämta filen från servern");
                
                const blob = await res.blob();
                const file = new File([blob], item.original_filename, { type: item.mime_type });
                
                // Hårdkodat minne att radera objektet vid bokföring
                this.form.inboxItemId = item.id;
                
                this.processDroppedFile(file);
                
                // Scrolla upp till formuläret i huvudvyn
                const mc = document.querySelector('.main-content');
                if (mc) mc.scrollTo({ top: 0, behavior: 'smooth' });
            } catch(err) {
                this.showToast(err.message, 'error');
            }
        },

        async handleFileDrop(e) {
            // Check for internal drop from the Inbox Drawer
            if (e.dataTransfer && Array.from(e.dataTransfer.types).includes('application/json')) {
                const dataStr = e.dataTransfer.getData('application/json');
                if (dataStr) {
                    const item = JSON.parse(dataStr);
                    await this.loadInboxItemToForm(item);
                }
                return;
            }

            const files = e.dataTransfer ? e.dataTransfer.files : e.target.files;
            if (!files || files.length === 0) return;
            const file = files[0];
            
            this.form.inboxItemId = null; // Rensa om ny extern fil droppas
            this.processDroppedFile(file);
        },

        processDroppedFile(file) {
            if (file.size > 20 * 1024 * 1024) {
                this.showToast("Filen är för stor (Max 20MB)", "error");
                return;
            }
            
            const validTypes = ['application/pdf', 'image/png', 'image/jpeg'];
            if (!validTypes.includes(file.type)) {
                this.showToast("Ogiltig filtyp. Endast PDF, PNG och JPEG tillåts.", "error");
                return;
            }

            const reader = new FileReader();
            reader.onload = async (event) => {
                const b64 = event.target.result.split(',')[1];
                this.form.attachmentBase64 = b64;
                this.form.attachmentName = file.name;
                this.form.attachmentMime = file.type;
                this.form.attachmentSize = this.formatBytes(file.size);
                
                if (this.form.attachmentUrl) {
                    URL.revokeObjectURL(this.form.attachmentUrl);
                }
                this.form.attachmentUrl = URL.createObjectURL(file);
                
                // Om filen droppas och kontot är 1930/3010, flytta fokus
                this.$nextTick(() => {
                    if (this.$refs.formDesc) this.$refs.formDesc.focus();
                });
                
                // OCR-Spiken: Starta avläsning omedelbart efter uppladdning
                await this.performOCR(file, event.target.result);
            };
            reader.readAsDataURL(file);
        },

        async performOCR(file, dataUrl) {
            this.isScanningOcr = true;
            this.ocrStatus = "Initierar OCR...";
            
            try {
                let imgSource = dataUrl;
                let finalRawText = "";
                
                if (file.type === 'application/pdf') {
                    this.ocrStatus = "Analyserar PDF...";
                    const buffer = await file.arrayBuffer();
                    const pdfResult = await this.rasterizePDF(buffer);
                    if (!pdfResult) throw new Error("Kunde inte läsa PDF.");
                    
                    if (pdfResult.isDirectText) {
                        finalRawText = pdfResult.text;
                    } else {
                        imgSource = pdfResult; // Rasteriserad bild
                    }
                } else {
                    this.ocrStatus = "Optimerar bild för OCR...";
                    imgSource = await this.downscaleImage(dataUrl);
                }

                if (!finalRawText) {
                    this.ocrStatus = "Tolkar bild (Tesseract WASM)...";
                    const worker = await Tesseract.createWorker('swe', 1, {
                        workerPath: '/static/js/vendor/ocr/worker.min.js',
                        corePath: '/static/js/vendor/ocr/tesseract-core.wasm.js',
                        langPath: '/static/js/vendor/ocr',
                        gzip: true,
                        logger: m => {
                            if (m.status === 'recognizing text') {
                                this.ocrStatus = `Tolkar text (${Math.round(m.progress * 100)}%)...`;
                            }
                        }
                    });
                    
                    const { data: { text } } = await worker.recognize(imgSource);
                    await worker.terminate();
                    finalRawText = text;
                }

                console.log("=== OCR RAW TEXT ===");
                console.log(finalRawText);
                
                // Slice 4: Inverted Matching & Heuristics (Backend)
                this.ocrStatus = "Slår upp leverantör och belopp...";
                const ocrRes = await this.authFetch('/api/ocr/parse', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ raw_text: finalRawText })
                });

                if (!ocrRes.ok) {
                    throw new Error(`Serverfel vid OCR-parsning: ${ocrRes.status}`);
                }

                const data = await ocrRes.json();
                console.log("=== OCR PARSED JSON ===", data);
                
                let fieldsFound = 0;

                // Destruktiv överskrivnings-guard
                if (data.vendor) {
                    fieldsFound++;
                    if (!this.form.text) {
                        this.form.text = data.vendor;
                    }
                }

                if (data.suggested_account) {
                    fieldsFound++;
                    let rowToUpdate = this.form.rows.find(r => r.account !== '1930' && r.account !== '1910' && r.account !== '2440' && r.account !== '1510' && !r.account.startsWith('26'));
                    if (!rowToUpdate) {
                        rowToUpdate = this.form.rows.find(r => !r.account);
                    }
                    if (rowToUpdate) {
                        rowToUpdate.account = data.suggested_account;
                        this.showToast(`Föreslog konto ${data.suggested_account} baserat på historik!`, 'success');
                    }
                }

                if (data.amount_cents > 0) {
                    fieldsFound++;
                    // Pre-fill the Magic Wand (Trollstaven) prompt amount, only if user hasn't typed in it yet.
                    if (!this.templatePrompt.amount) {
                        this.templatePrompt.amount = data.amount_cents / 100;
                    }
                }

                if (data.date) {
                    fieldsFound++;
                    // BFL-Regel: Kvittots datum är auktoritativt. Men vi varnar användaren om vi skriver över deras manuella (Dagens) datum.
                    if (this.form.date && this.form.date !== data.date) {
                        this.showToast(`Datum uppdaterades automatiskt till kvittots datum (${data.date})`, 'warning');
                    }
                    this.form.date = data.date;
                    
                    // Fiscal Year Validation Warning
                    const fy = this.activeFiscalYear;
                    if (fy && (data.date < fy.start_date || data.date > fy.end_date)) {
                        setTimeout(() => this.showToast(`Kvittots datum (${data.date}) ligger utanför aktivt räkenskapsår!`, 'warning'), 1500);
                    }
                }

                // Dynamisk UX-Feedback
                if (data.currency === "FOREIGN") {
                    this.showToast("Utländsk valuta upptäckt! Kontrollera beloppet manuellt.", "warning");
                } else if (fieldsFound === 3) {
                    this.showToast("OCR klar! Datum, belopp och leverantör ifyllt.", "success");
                } else if (fieldsFound > 0) {
                    this.showToast("OCR delvis lyckad. Vänligen komplettera saknade fält.", "warning");
                } else {
                    this.showToast("OCR kunde inte tolka texten. Fyll i manuellt.", "error"); // Bytt till orange/error
                }
                
            } catch (e) {
                console.error("OCR Error:", e);
                this.showToast("OCR misslyckades: " + e.message, "error");
            } finally {
                this.isScanningOcr = false;
                this.ocrStatus = "";
            }
        },

        downscaleImage(dataUrl) {
            return new Promise((resolve) => {
                const img = new Image();
                img.onload = () => {
                    const canvas = document.createElement('canvas');
                    const MAX_WIDTH = 1500;
                    let width = img.width;
                    let height = img.height;
                    
                    if (width > MAX_WIDTH) {
                        height = Math.round((height * MAX_WIDTH) / width);
                        width = MAX_WIDTH;
                    }
                    
                    canvas.width = width;
                    canvas.height = height;
                    const ctx = canvas.getContext('2d');
                    
                    // Fyll med vit bakgrund ifall bilden har transparens
                    ctx.fillStyle = "#ffffff";
                    ctx.fillRect(0, 0, width, height);
                    ctx.drawImage(img, 0, 0, width, height);
                    resolve(canvas.toDataURL('image/jpeg', 0.9));
                };
                img.onerror = () => {
                    console.warn("Kunde inte ladda bilden för optimering. Fortsätter med originalbild.");
                    resolve(dataUrl);
                };
                img.src = dataUrl;
            });
        },

        async rasterizePDF(buffer) {
            if (!window.pdfjsLib) {
                console.warn("PDF.js inte laddad. Fallback ignoreras.");
                return null;
            }
            
            const pdfjsLib = window.pdfjsLib;
            pdfjsLib.GlobalWorkerOptions.workerSrc = '/static/js/vendor/ocr/pdf.worker.min.js';
            
            const loadingTask = pdfjsLib.getDocument({ data: new Uint8Array(buffer) });
            const pdf = await loadingTask.promise;
            const page = await pdf.getPage(1);
            
            // Försök extrahera vektor-text först
            const textContent = await page.getTextContent();
            const rawText = textContent.items.map(item => item.str).join(' ');
            
            if (rawText.trim().length > 20) {
                console.log("PDF.js extraherade text direkt!");
                return { isDirectText: true, text: rawText };
            }
            
            console.log("PDF.js text är tom. Rasteriserar för Tesseract fallback...");
            
            // Rasterisera inscannad PDF
            const scale = 2.0; 
            const viewport = page.getViewport({ scale: scale });
            const canvas = document.createElement('canvas');
            const ctx = canvas.getContext('2d');
            canvas.height = viewport.height;
            canvas.width = viewport.width;
            
            const renderContext = { canvasContext: ctx, viewport: viewport };
            await page.render(renderContext).promise;
            
            return canvas.toDataURL('image/jpeg', 0.9);
        },

        inspectSavedReceipt(hash, mime) {
            if (!hash) return;
            this.inspector.mime = mime;
            this.inspector.url = '/api/attachments/' + hash;
            this.showInspector = true;
        },

        inspectCurrentReceipt() {
            if (!this.form.attachmentUrl) return;
            this.inspector.mime = this.form.attachmentMime;
            this.inspector.url = this.form.attachmentUrl;
            this.showInspector = true;
        },

        removeAttachment() {
            if (this.form.attachmentUrl) {
                URL.revokeObjectURL(this.form.attachmentUrl);
            }
            this.form.attachmentBase64 = '';
            this.form.attachmentName = '';
            this.form.attachmentMime = '';
            this.form.attachmentUrl = '';
            this.showInspector = false;
        },

        addRow() {
            this.form.rows.push({ _key: Date.now(), account: '', debet: 0, kredit: 0 });
        },

        removeRow(index) {
            if (this.form.rows.length > 2) {
                this.form.rows.splice(index, 1);
            }
        },

        getTotalDebet() {
            return this.form.rows.reduce((sum, row) => {
                let val = row.debet;
                if (typeof val === 'string') val = val.replace(',', '.').replace(/ /g, '');
                let num = parseFloat(val);
                return sum + (isNaN(num) ? 0 : num);
            }, 0);
        },

        getTotalKredit() {
            return this.form.rows.reduce((sum, row) => {
                let val = row.kredit;
                if (typeof val === 'string') val = val.replace(',', '.').replace(/ /g, '');
                let num = parseFloat(val);
                return sum + (isNaN(num) ? 0 : num);
            }, 0);
        },

        getDiff() {
            return this.getTotalDebet() - this.getTotalKredit();
        },

        focusNext(event) {
            const section = event.target.closest('section');
            if (!section) return;
            const inputs = Array.from(section.querySelectorAll('.numpad-nav'));
            const idx = inputs.indexOf(event.target);
            if (idx === -1) return;

            if (idx < inputs.length - 1) {
                // Focus the next input
                const nextInput = inputs[idx + 1];
                nextInput.focus();
                if (nextInput.type !== 'date' && typeof nextInput.select === 'function') {
                    nextInput.select();
                }
            } else {
                // Last input (kredit row), add new row
                this.addRow();
                // Then focus the newly added account field in the next tick
                this.$nextTick(() => {
                    const newInputs = Array.from(section.querySelectorAll('.numpad-nav'));
                    // The new row adds 3 inputs: account, debet, kredit. We want to focus the account field, which is at length - 3
                    if (newInputs.length >= 3) {
                        newInputs[newInputs.length - 3].focus();
                    }
                });
            }
        },

        // Command Palette Handlers
        async showView(viewName) {
            this.currentView = viewName;
            this.showDashboard = false;
            this.showMoms = false;
            this.showKontoplan = false;
            this.showSettings = false;
            this.showInvoices = false;
            this.showTools = false;
            this.showReports = false;
            this.showHelp = false;
            this.isInboxDrawerOpen = false;

            switch(viewName) {
                case 'dashboard':
                    this.showDashboard = true;
                    break;
                case 'momsredovisning':
                    this.showMoms = true;
                    break;
                case 'kontoplan':
                    this.showKontoplan = true;
                    break;
                case 'installningar':
                    this.showSettings = true;
                    if (this.customersStale || this.customers.length === 0) {
                        await this.fetchCustomers();
                        this.customersStale = false;
                    }
                    break;
                case 'verktyg':
                    this.showTools = true;
                    break;
                case 'rapporter':
                    this.showReports = true;
                    this.loadFinancialReport();
                    break;
                case 'help':
                    this.showHelp = true;
                    break;
                case 'fakturering':
                    this.showInvoices = true;
                    this.loadInvoices();
                    if (this.customersStale || this.customers.length === 0) {
                        await this.fetchCustomers();
                        this.customersStale = false;
                    }
                    break;
                case 'huvudbok':
                    // Default view is huvudbok when all others are false
                    break;
            }
        },

        handleCmdAction(actionName) {
            if (actionName === 'ny-verifikation') {
                this.showView('huvudbok');
                // Focus the description input field to start typing immediately
                setTimeout(() => {
                    const descInput = document.querySelector('[x-ref="formDesc"]');
                    if (descInput) descInput.focus();
                }, 100);
            }
        },

        async submitPost() {
            if (this.showSettings || this.showKontoplan || this.showMoms || this.isScanningOcr) return;

            if (!this.form.date || !this.form.text) {
                this.showToast('Datum och text är obligatoriskt', 'error');
                return;
            }

            const fy = this.activeFiscalYear;
            if (fy && (this.form.date < fy.start_date || this.form.date > fy.end_date)) {
                this.formDateError = 'Datumet ligger utanför det aktiva räkenskapsåret';
                return;
            }
            this.formDateError = '';

            if (this.getDiff() !== 0) {
                this.showToast('Verifikationen balanserar inte!', 'error');
                return;
            }
            if (this.getTotalDebet() <= 0) {
                this.showToast('Du måste ange ett belopp över 0 kronor', 'error');
                return;
            }

            // Filtrera bort helt tomma rader (men spara rader med ogiltiga/negativa belopp så backend kan varna om dem)
            const validRows = this.form.rows.filter(r => r.account && (r.debet !== 0 || r.kredit !== 0 || r.debet < 0 || r.kredit < 0));

            try {
                const reqBody = {
                    date: this.form.date,
                    text: this.form.text,
                    attachmentBase64: this.form.attachmentBase64,
                    rows: validRows.map(r => ({
                        account: r.account.toString().trim(),
                        debet: Math.round((Number(typeof r.debet === 'string' ? r.debet.replace(',', '.') : r.debet) || 0) * 100),
                        kredit: Math.round((Number(typeof r.kredit === 'string' ? r.kredit.replace(',', '.') : r.kredit) || 0) * 100)
                    }))
                };
                
                const res = await this.authFetch('/api/verifications', {
                    method: 'POST',
                    body: JSON.stringify(reqBody)
                });

                if (res.ok) {
                    this.showToast('Bokförd!', 'success');
                    
                    // Om det kom från inkorgen, radera det därifrån nu
                    if (this.form.inboxItemId) {
                        await this.deleteInboxItem(this.form.inboxItemId);
                        this.form.inboxItemId = null;
                        this.isInboxDrawerOpen = true; // Öppna lådan igen efter bokföring
                    }
                    
                    // Uppdatera dashboard lokalt
                    this.updateDashboardLocally(validRows);
                    
                    // Reset form
                    this.form.text = '';
                    this.removeAttachment();
                    this.form.rows = [
                        { _key: Date.now() + 1, account: '1930', debet: 0, kredit: 0 },
                        { _key: Date.now() + 2, account: '3010', debet: 0, kredit: 0 }
                    ];
                    this.showInspector = false;
                    await this.fetchVerifications();
                    await this.fetchDashboardData();
                    await this.runVerification();
                    
                    // UX: Scrolla ner så att användaren direkt ser den nya posten i listan
                    setTimeout(() => {
                        const mc = document.querySelector('.main-content');
                        if (mc) mc.scrollTo({ top: mc.scrollHeight, behavior: 'smooth' });
                    }, 100);
                } else {
                    const err = await res.json();
                    this.showToast('Fel: ' + err.error, 'error');
                }
            } catch (e) {
                this.showToast('Ett oväntat fel uppstod', 'error');
            }
        },

        showToast(msg, type) {
            if (this._toastTimeout) {
                clearTimeout(this._toastTimeout);
                this._toastTimeout = null;
            }
            this.toast = { msg, type };
            const duration = (type === 'error' || type === 'warning') ? 10000 : 3000;
            this._toastTimeout = setTimeout(() => {
                this.toast = null;
                this._toastTimeout = null;
            }, duration);
        },

        dismissToast() {
            if (this._toastTimeout) {
                clearTimeout(this._toastTimeout);
                this._toastTimeout = null;
            }
            this.toast = null;
        },

        copyBugLog() {
            navigator.clipboard.writeText(this.bugLog.join('\n'));
            this.showToast('Logg kopierad', 'success');
        },

        clearBugLog() {
            this.bugLog = [];
            this.showBugLogger = false;
            this.showToast('Logg rensad', 'success');
        },

        acceptEula() {
            localStorage.setItem('eula_accepted_version', this.currentEulaVersion);
            this.eulaAcceptedVersion = this.currentEulaVersion;
            this.showEulaModal = false;
            this.showToast('EULA-avtalet har godkänts!', 'success');
            this.completeInit();
        },

        async completeInit() {
            await this.fetchFiscalYears();
            this.syncFormDateWithFiscalYear();
            this.fetchAccounts();
            this.fetchVerifications();
            this.fetchDashboardData();
            this.fetchSettings();
            this.runVerification();
            this.fetchInbox();
            this.loadInvoices();
            this.fetchCustomers();
        },

        copyEmailFallback() {
            navigator.clipboard.writeText('dka120@hotmail.com');
            this.showToast('E-postadressen dka120@hotmail.com har kopierats till urklipp!', 'success');
        }
    }));

    Alpine.data('inlineConfirm', (actionId, actionCallback) => ({
        timer: null,
        cooldown: false,
        
        get confirming() {
            return this.armedActionId === actionId;
        },
        
        handleClick() {
            if (!this.confirming) {
                this.armedActionId = actionId;
                this.cooldown = true;
                setTimeout(() => this.cooldown = false, 250);
                
                this.timer = setTimeout(() => {
                    if (this.armedActionId === actionId) this.armedActionId = null;
                }, 3000);
            } else {
                if (this.cooldown) return;
                
                this.armedActionId = null;
                clearTimeout(this.timer);
                actionCallback();
            }
        },
        
        cancel() {
            if (this.confirming) {
                this.armedActionId = null;
                clearTimeout(this.timer);
            }
        }
    }));
});






