document.addEventListener('alpine:init', () => {
    Alpine.data('toolsApp', () => ({
        token: '',
        fiscalYears: [],
        
        // Form states
        selectedExportYear: '',
        ibSourceYear: '',
        ibTargetYearName: '',
        importSieYear: '',
        
        // Files
        selectedSieFile: null,
        selectedBackupFile: null,
        
        // Loading states
        isGeneratingIB: false,
        isUploadingSie: false,
        isUploadingBackup: false,
        isExportingBackup: false,
        
        // SIE Preview
        showSiePreviewModal: false,
        siePreview: null,
        
        // Toast
        toast: null,
        toastTimeout: null,

        init() {
            // Läs in den injicerade tokenen
            const metaToken = document.querySelector('meta[name="api-token"]');
            if (metaToken) {
                this.token = metaToken.getAttribute('content');
            }
            this.fetchFiscalYears();

            // Reaktiv watcher för ibTargetYearName
            this.$watch('ibSourceYear', (val) => {
                if (!val) {
                    this.ibTargetYearName = '';
                    return;
                }
                const source = this.fiscalYears.find(f => f.id == val);
                if (!source) {
                    this.ibTargetYearName = '';
                    return;
                }
                const sourceEnd = new Date(source.end_date);
                const target = this.fiscalYears.find(f => new Date(f.start_date) > sourceEnd);
                this.ibTargetYearName = target ? target.label : '';
            });
        },

        showToast(msg, type = 'info') {
            this.toast = { message: msg, type: type };
            if (this.toastTimeout) clearTimeout(this.toastTimeout);
            this.toastTimeout = setTimeout(() => {
                this.toast = null;
            }, 4000);
        },

        async authFetch(url, options = {}) {
            if (!options.headers) options.headers = {};
            if (this.token) {
                options.headers['Authorization'] = `Bearer ${this.token}`;
            }
            return fetch(url, options);
        },

        async fetchFiscalYears() {
            try {
                const res = await this.authFetch('/api/fiscal-years');
                if (res.ok) {
                    const data = await res.json();
                    
                    this.fiscalYears = (data || [])
                        .sort((a, b) => new Date(a.start_date) - new Date(b.start_date))
                        .map(fy => {
                            const startY = fy.start_date?.substring(0, 4) ?? 'Okänt';
                            const endY = fy.end_date?.substring(0, 4) ?? 'Okänt';
                            return {
                                ...fy,
                                label: startY === endY ? startY : `${startY}/${endY}`
                            };
                        });
                    
                    // Sätt defaultvärden om det finns år
                    if (this.fiscalYears.length > 0) {
                        this.selectedExportYear = this.fiscalYears[0].id;
                        this.importSieYear = this.fiscalYears[0].id;
                    }
                }
            } catch (e) {
                this.showToast("Kunde inte ladda räkenskapsår", "error");
            }
        },

        // --- EXPORT SIE-4 ---
        async exportSIE4() {
            if (!this.selectedExportYear) return;
            try {
                const res = await this.authFetch(`/api/export/sie4?year_id=${this.selectedExportYear}`);
                if (!res.ok) {
                    this.showToast('Misslyckades att exportera SIE-4', 'error');
                    return;
                }
                const blob = await res.blob();
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = `LocalLedger_Export_${this.selectedExportYear}.se`;
                document.body.appendChild(a);
                a.click();
                a.remove();
                window.URL.revokeObjectURL(url);
            } catch (e) {
                this.showToast('Nätverksfel vid export', 'error');
            }
        },

        // --- DATABAS BACKUP ---
        // Konsoliderad: använder nu globala ledgerApp-metoder i app.js


        // --- ÅRSAVSLUT (IB) ---

        async generateIB() {
            if (!this.ibSourceYear) return;
            
            const source = this.fiscalYears.find(f => f.id == this.ibSourceYear);
            const sourceEnd = new Date(source.end_date);
            const target = this.fiscalYears.find(f => new Date(f.start_date) > sourceEnd);

            if (!target) {
                this.showToast('Målår saknas. Skapa ett nytt räkenskapsår först.', 'error');
                return;
            }
            
            if (!confirm(`Är du säker på att du vill stänga '${source.label}' och kopiera saldon till '${target.label}'? Detta skapar en IB-verifikation i målåret.`)) {
                return;
            }

            this.isGeneratingIB = true;
            try {
                const res = await this.authFetch(`/api/fiscal-years/${target.id}/generate-ib`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ from_year_id: parseInt(source.id) })
                });
                if (res.ok) {
                    this.showToast(`Årsavslut klart! IB är genererad i ${target.label}.`, 'success');
                    this.ibSourceYear = ''; // Nollställ
                } else {
                    const err = await res.json();
                    this.showToast(`Misslyckades: ${err.error}`, 'error');
                }
            } catch (e) {
                this.showToast('Nätverksfel vid årsavslut', 'error');
            } finally {
                this.isGeneratingIB = false;
            }
        },

        // --- SIE IMPORT ---
        handleSieSelected(e) {
            const files = e.target.files;
            if (files && files.length > 0) {
                this.selectedSieFile = files[0];
            }
        },

        async uploadSieFile() {
            if (!this.selectedSieFile || !this.importSieYear) return;

            this.isUploadingSie = true;
            this.showToast('Analyserar SIE-fil...', 'info');
            const formData = new FormData();
            formData.append('file', this.selectedSieFile);
            formData.append('yearID', this.importSieYear);
            formData.append('dry_run', 'true');

            try {
                const res = await this.authFetch('/api/import/sie4?dry_run=true', {
                    method: 'POST',
                    body: formData
                });
                
                if (res.ok) {
                    this.siePreview = await res.json();
                    this.showSiePreviewModal = true;
                } else {
                    const err = await res.json();
                    this.showToast(`Analys misslyckades: ${err.error}`, 'error');
                }
            } catch(e) {
                this.showToast('Nätverksfel', 'error');
            } finally {
                this.isUploadingSie = false;
            }
        },

        async confirmImportSie() {
            if (!this.selectedSieFile || !this.importSieYear) return;

            this.isUploadingSie = true;
            this.showToast('Importerar verifikationer...', 'info');
            const formData = new FormData();
            formData.append('file', this.selectedSieFile);
            formData.append('yearID', this.importSieYear);

            try {
                const res = await this.authFetch('/api/import/sie4', {
                    method: 'POST',
                    body: formData
                });
                
                if (res.ok) {
                    this.showToast('SIE-filen har importerats framgångsrikt!', 'success');
                    this.selectedSieFile = null;
                    this.$refs.sieFileInput.value = '';
                    this.showSiePreviewModal = false;
                    this.siePreview = null;
                } else {
                    const err = await res.json();
                    this.showToast(`Import misslyckades: ${err.error}`, 'error');
                }
            } catch(e) {
                this.showToast('Nätverksfel vid import', 'error');
            } finally {
                this.isUploadingSie = false;
            }
        },

        // --- BACKUP RESTORE ---
        // Konsoliderad: använder nu globala ledgerApp-metoder i app.js

    }));
});






