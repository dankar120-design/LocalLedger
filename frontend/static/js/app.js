document.addEventListener('alpine:init', () => {
    Alpine.data('ledgerApp', () => ({
        token: '',
        verifications: [],
        searchQuery: '',
        verifyStatus: 'Checking...',
        verifyOk: true,
        toast: null,
        fiscalYears: [],
        activeFiscalYearId: null,
        showInspector: false,
        showSettings: false,
        showKontoplan: false,
        showMoms: false,
        showSealModal: false,
        isSealing: false,
        vatReport: null,
        vatStartDate: '',
        vatEndDate: '',
        vatPeriodPreset: '',
        basAccounts: [],
        settings: { name: '', org_number: '' },
        newAccount: { code: '', name: '', type: 'Kostnad' },
        
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
                accountBase: '3041'   // Kredit
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

        applyTemplate() {
            if (!this.selectedTemplate) return;
            
            const t = this.templates.find(x => x.id === this.selectedTemplate);
            this.selectedTemplate = ''; // Återställ dropdown direkt
            
            if (!t) return;
            
            const inputStr = window.prompt(`Ange totalbelopp inkl. eventuell moms för "${t.name}" (kr):`);
            if (!inputStr) return; // Användaren avbröt
            
            // Hantera komma och punkt
            const cleaned = inputStr.replace(',', '.').replace(/ /g, '');
            const amountFloat = parseFloat(cleaned);
            if (isNaN(amountFloat) || amountFloat <= 0) {
                this.showToast("Ogiltigt belopp", "error");
                return;
            }
            
            // Remainder-metoden (räkna i ören)
            const totalOren = Math.round(amountFloat * 100);
            
            let vatOren = 0;
            let baseOren = totalOren;
            
            if (t.vatRate > 0) {
                // Momssumma baklänges = Total * (moms / (1 + moms))
                const vatFraction = t.vatRate / (1 + t.vatRate);
                vatOren = Math.round(totalOren * vatFraction);
                baseOren = totalOren - vatOren;
            }
            
            // Skapa raderna
            if (!this.form.text) this.form.text = t.desc;
            this.form.rows = []; // Rensa befintliga
            
            if (t.type === 'expense') {
                this.form.rows.push({ account: t.accountTotal, debet: 0, kredit: totalOren / 100 });
                if (vatOren > 0) this.form.rows.push({ account: t.accountVat, debet: vatOren / 100, kredit: 0 });
                this.form.rows.push({ account: t.accountBase, debet: baseOren / 100, kredit: 0 });
            } else if (t.type === 'income') {
                this.form.rows.push({ account: t.accountTotal, debet: totalOren / 100, kredit: 0 });
                if (vatOren > 0) this.form.rows.push({ account: t.accountVat, debet: 0, kredit: vatOren / 100 });
                this.form.rows.push({ account: t.accountBase, debet: 0, kredit: baseOren / 100 });
            } else if (t.type === 'transfer' && t.id === 'egen_insattning') {
                this.form.rows.push({ account: t.accountTotal, debet: totalOren / 100, kredit: 0 });
                this.form.rows.push({ account: t.accountBase, debet: 0, kredit: totalOren / 100 });
            } else if (t.type === 'transfer' && t.id === 'eget_uttag') {
                this.form.rows.push({ account: t.accountTotal, debet: 0, kredit: totalOren / 100 });
                this.form.rows.push({ account: t.accountBase, debet: totalOren / 100, kredit: 0 });
            }
            
            this.showToast("Mall tillämpad", "success");
        },
        
        form: {
            date: new Date().toISOString().split('T')[0],
            text: '',
            attachmentBase64: '',
            attachmentName: '',
            attachmentMime: '',
            attachmentUrl: '',
            rows: [
                { account: '1930', debet: 0, kredit: 0 },
                { account: '3010', debet: 0, kredit: 0 }
            ]
        },

        async init() {
            // Read token from DOM injection
            const meta = document.querySelector('meta[name="api-token"]');
            if (meta) {
                this.token = meta.getAttribute('content');
            } else {
                this.showToast('Authentication token missing!', 'error');
            }

            await this.fetchFiscalYears();
            this.fetchAccounts();
            this.fetchVerifications();
            this.fetchSettings();
            this.runVerification();
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
            if (!confirm("Vill du stänga av LocalLedger?")) return;
            try {
                await this.authFetch('/api/shutdown', { method: 'POST' });
                document.body.innerHTML = `
                    <div style="display:flex; height:100vh; align-items:center; justify-content:center; background:#0f172a; color:white; font-family:var(--font-sans);">
                        <div style="text-align:center;">
                            <h1>System Avstängt</h1>
                            <p>Du kan nu stänga denna flik.</p>
                        </div>
                    </div>
                `;

            } catch (e) {
                console.error("Failed to shutdown", e);
                this.showToast("Kunde inte stänga av servern", "error");
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
            if (!confirm("Är du säker på att du vill stänga detta år och föra över balanser till ett nytt räkenskapsår?")) return;
            try {
                const res = await this.authFetch('/api/fiscal-years', { method: 'POST' });
                if (res.ok) {
                    this.showToast('Nytt räkenskapsår skapat framgångsrikt', 'success');
                    await this.fetchFiscalYears();
                    this.activeFiscalYearId = this.fiscalYears[0].id; // Välj det nya
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
            if (!confirm("VARNING: Att låsa ett räkenskapsår är permanent. Inga fler ändringar eller makuleringar kan göras. Fortsätt?")) return;
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
                    // Sorterna fallande (nyaste först) om data inte redan är det
                    this.verifications = data || [];
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
            if (!confirm(`Vill du verkligen stornera verifikation A${id}? En ny rättelsepost kommer att skapas med dagens datum.`)) return;
            try {
                const res = await this.authFetch(`/api/verifications/${id}/storno`, { method: 'POST' });
                if (res.ok) {
                    this.showToast('Verifikation stornerad framgångsrikt', 'success');
                    this.fetchVerifications();
                } else {
                    const err = await res.json();
                    this.showToast('Fel vid stornering: ' + err.error, 'error');
                }
            } catch (e) {
                this.showToast('Nätverksfel vid stornering', 'error');
            }
        },

        async voidDraft(id) {
            if (!confirm(`Vill du verkligen makulera utkast A${id}? Beloppen kommer att nollställas.`)) return;
            try {
                const res = await this.authFetch(`/api/verifications/${id}/void`, { method: 'POST' });
                if (res.ok) {
                    this.showToast('Utkast makulerat', 'success');
                    this.fetchVerifications();
                } else {
                    const err = await res.json();
                    this.showToast('Fel vid makulering: ' + err.error, 'error');
                }
            } catch (e) {
                this.showToast('Nätverksfel', 'error');
            }
        },

        async sealVerifications() {
            try {
                const res = await this.authFetch('/api/maintenance/seal', { method: 'POST' });
                if (res.ok) {
                    this.showToast('Utkast låsta och WORM-kedjan är säkrad', 'success');
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

        async transferVat() {
            if (!this.vatReport) return;
            if (this.vatReport.net_vat === 0) {
                this.showToast('Finns inget att omföra (Nettot är 0).', 'error');
                return;
            }
            if (!confirm(`Är du säker? Detta skapar en verifikation som nollställer moms och därefter LÅSER alla månader i perioden (${this.vatStartDate} till ${this.vatEndDate}) för framtida ändringar.`)) return;

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
                this.fetchData();
            } catch (e) {
                this.showToast('Fel vid momsomföring: ' + e.message, 'error');
            }
        },

        setVatPreset(preset) {
            this.vatPeriodPreset = preset;
            const currentYear = this.activeFiscalYearId ? this.fiscalYears.find(y => y.id === this.activeFiscalYearId)?.start_date.substring(0, 4) : new Date().getFullYear();
            
            if (preset === 'Q1') { this.vatStartDate = `${currentYear}-01-01`; this.vatEndDate = `${currentYear}-03-31`; }
            else if (preset === 'Q2') { this.vatStartDate = `${currentYear}-04-01`; this.vatEndDate = `${currentYear}-06-30`; }
            else if (preset === 'Q3') { this.vatStartDate = `${currentYear}-07-01`; this.vatEndDate = `${currentYear}-09-30`; }
            else if (preset === 'Q4') { this.vatStartDate = `${currentYear}-10-01`; this.vatEndDate = `${currentYear}-12-31`; }
            else if (preset === 'Year') { this.vatStartDate = `${currentYear}-01-01`; this.vatEndDate = `${currentYear}-12-31`; }
            
            if (this.vatStartDate && this.vatEndDate) {
                this.calculateVatReport();
            }
        },

        get netVatText() {
            if (!this.vatReport) return '';
            return this.vatReport.net_vat > 0 ? 'Att Betala (Skuld till SKV)' : 'Få Tillbaka (Fordran)';
        },

        async fillGaps() {
            if (!confirm('Vill du leta upp luckor i nummerserien och fylla dem med makuleringsposter (Enligt BFL)?')) return;
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

        handleFileDrop(e) {
            const files = e.dataTransfer ? e.dataTransfer.files : e.target.files;
            if (!files || files.length === 0) return;
            const file = files[0];
            
            if (file.size > 10 * 1024 * 1024) {
                this.showToast("Filen är för stor (Max 10MB)", "error");
                return;
            }
            
            const validTypes = ['application/pdf', 'image/png', 'image/jpeg'];
            if (!validTypes.includes(file.type)) {
                this.showToast("Ogiltig filtyp. Endast PDF, PNG och JPEG tillåts.", "error");
                return;
            }

            const reader = new FileReader();
            reader.onload = (event) => {
                const b64 = event.target.result.split(',')[1];
                this.form.attachmentBase64 = b64;
                this.form.attachmentName = file.name;
                this.form.attachmentMime = file.type;
                this.form.attachmentUrl = URL.createObjectURL(file);
                this.showInspector = true;
                this.showToast("Bilaga bifogad: " + file.name, "success");
            };
            reader.readAsDataURL(file);
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
            this.form.rows.push({ account: '', debet: 0, kredit: 0 });
        },

        removeRow(index) {
            if (this.form.rows.length > 2) {
                this.form.rows.splice(index, 1);
            }
        },

        get totalDebet() {
            return this.form.rows.reduce((sum, row) => sum + (Number(row.debet) || 0), 0);
        },

        get totalKredit() {
            return this.form.rows.reduce((sum, row) => sum + (Number(row.kredit) || 0), 0);
        },

        get diff() {
            return this.totalDebet - this.totalKredit;
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

        async submitPost() {
            if (this.showSettings || this.showKontoplan || this.showMoms) return;

            if (this.diff !== 0) {
                this.showToast('Verifikationen balanserar inte!', 'error');
                return;
            }
            if (!this.form.date || !this.form.text) {
                this.showToast('Datum och text är obligatoriskt', 'error');
                return;
            }

            // Filtrera bort helt tomma rader (men spara rader med ogiltiga/negativa belopp så backend kan varna om dem)
            const validRows = this.form.rows.filter(r => r.account && (r.debet !== 0 || r.kredit !== 0 || r.debet < 0 || r.kredit < 0));

            try {
                const res = await this.authFetch('/api/verifications', {
                    method: 'POST',
                    body: JSON.stringify({
                        date: this.form.date,
                        text: this.form.text,
                        attachmentBase64: this.form.attachmentBase64,
                        rows: validRows.map(r => ({
                            account: r.account.toString().trim(),
                            debet: Math.round(Number(r.debet) * 100), // Till öre om vi använde float, men vi antar heltal nu
                            kredit: Math.round(Number(r.kredit) * 100)
                        }))
                    })
                });

                if (res.ok) {
                    this.showToast('Bokförd!', 'success');
                    // Reset form
                    this.form.text = '';
                    this.removeAttachment();
                    this.form.rows = [
                        { account: '1930', debet: 0, kredit: 0 },
                        { account: '3010', debet: 0, kredit: 0 }
                    ];
                    this.showInspector = false;
                    this.fetchVerifications();
                    this.runVerification();
                } else {
                    const err = await res.json();
                    this.showToast('Fel: ' + err.error, 'error');
                }
            } catch (e) {
                this.showToast('Ett oväntat fel uppstod', 'error');
            }
        },

        showToast(msg, type) {
            this.toast = { msg, type };
            setTimeout(() => { this.toast = null; }, 3000);
        }
    }));
});
