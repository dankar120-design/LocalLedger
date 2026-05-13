document.addEventListener('alpine:init', () => {
    Alpine.data('toolsApp', () => ({
        token: '',
        fiscalYears: [],
        
        // Form states
        selectedExportYear: '',
        ibSourceYear: '',
        importSieYear: '',
        
        // Files
        selectedSieFile: null,
        selectedBackupFile: null,
        
        // Loading states
        isGeneratingIB: false,
        isUploadingSie: false,
        isUploadingBackup: false,
        isExportingBackup: false,
        
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
        async exportBackup() {
            this.isExportingBackup = true;
            try {
                const res = await this.authFetch('/api/export/backup');
                if (!res.ok) {
                    this.showToast('Misslyckades att exportera backup', 'error');
                    return;
                }
                const blob = await res.blob();
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = `localledger_backup.db`;
                document.body.appendChild(a);
                a.click();
                a.remove();
                window.URL.revokeObjectURL(url);
            } catch (e) {
                this.showToast('Nätverksfel vid export', 'error');
            } finally {
                this.isExportingBackup = false;
            }
        },

        // --- ÅRSAVSLUT (IB) ---
        get ibTargetYearName() {
            if (!this.ibSourceYear) return '';
            const source = this.fiscalYears.find(f => f.id == this.ibSourceYear);
            if (!source) return '';
            
            // Leta efter ett år vars start_date är >= source.end_date
            const sourceEnd = new Date(source.end_date);
            const target = this.fiscalYears.find(f => new Date(f.start_date) > sourceEnd);
            
            if (target) {
                return target.label;
            }
            return '';
        },

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

            if (!confirm(`Varning: Detta kommer importera '${this.selectedSieFile.name}' till det valda räkenskapsåret. Fortsätta?`)) {
                return;
            }

            this.isUploadingSie = true;
            const formData = new FormData();
            formData.append('file', this.selectedSieFile);
            formData.append('yearID', this.importSieYear); // Måste vara yearID (inte year_id) för backend

            try {
                // Notera: Content-Type sätts automatiskt av browsern för FormData
                const res = await this.authFetch('/api/import/sie4', {
                    method: 'POST',
                    body: formData
                });
                
                if (res.ok) {
                    this.showToast('SIE-filen har importerats framgångsrikt!', 'success');
                    this.selectedSieFile = null;
                    this.$refs.sieFileInput.value = '';
                } else {
                    const err = await res.json();
                    this.showToast(`Import misslyckades: ${err.error}`, 'error');
                }
            } catch(e) {
                this.showToast('Nätverksfel', 'error');
            } finally {
                this.isUploadingSie = false;
            }
        },

        // --- BACKUP RESTORE ---
        handleBackupSelected(e) {
            const files = e.target.files;
            if (files && files.length > 0) {
                this.selectedBackupFile = files[0];
            }
        },

        async uploadBackupFile() {
            if (!this.selectedBackupFile) return;

            if (!confirm(`EXTREM FARA: Du är på väg att skriva över HELA databasen med '${this.selectedBackupFile.name}'.\n\nAll nuvarande data kommer att raderas.\n\nÄR DU HELT SÄKER?`)) {
                return;
            }

            this.isUploadingBackup = true;
            const formData = new FormData();
            formData.append('database', this.selectedBackupFile);

            try {
                const res = await this.authFetch('/api/import/backup', {
                    method: 'POST',
                    body: formData
                });
                
                if (res.ok) {
                    alert('Databasen har återställts framgångsrikt!\n\nSystemet måste startas om eller sidan måste laddas om för att ändringarna ska slå igenom.');
                    // Tvinga hard reload för att återställa state
                    window.location.href = '/'; 
                } else {
                    const err = await res.text();
                    this.showToast(`Återställning misslyckades: ${err}`, 'error');
                }
            } catch(e) {
                this.showToast('Nätverksfel vid återställning', 'error');
            } finally {
                this.isUploadingBackup = false;
                this.selectedBackupFile = null;
                if (this.$refs.backupFileInput) this.$refs.backupFileInput.value = '';
            }
        }
    }));
});
