document.addEventListener('DOMContentLoaded', () => {
    // 1. Skapa DOM-strukturen
    const cpHTML = `
        <div id="cmd-palette-backdrop" class="cmd-hidden">
            <div id="cmd-palette-modal">
                <div id="cmd-palette-input-wrapper">
                    <svg class="cmd-search-icon" viewBox="0 0 24 24">
                        <circle cx="11" cy="11" r="8"></circle>
                        <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
                    </svg>
                    <input type="text" id="cmd-palette-input" placeholder="Vad vill du göra? (Sök kommando...)" autocomplete="off" spellcheck="false" />
                </div>
                <ul id="cmd-palette-results"></ul>
            </div>
        </div>
    `;
    document.body.insertAdjacentHTML('beforeend', cpHTML);

    const backdrop = document.getElementById('cmd-palette-backdrop');
    const input = document.getElementById('cmd-palette-input');
    const resultsList = document.getElementById('cmd-palette-results');
    
    let isOpen = false;
    let activeIndex = 0;
    let currentResults = [];

    // 2. Definiera Kommandon
    const commands = [
        { id: 'nav-dashboard', icon: '🏠', label: 'Gå till Dashboard', action: () => executeNav('dashboard') },
        { id: 'nav-huvudbok', icon: '📖', label: 'Gå till Huvudbok', action: () => executeNav('huvudbok') },
        { id: 'nav-samlingsplan', icon: '📄', label: 'Gå till Samlingsplan', action: () => window.open('/api/reports/samlingsplan', '_blank') },
        { id: 'nav-moms', icon: '📊', label: 'Gå till Momsredovisning', action: () => executeNav('momsredovisning') },
        { id: 'nav-kontoplan', icon: '🗂️', label: 'Gå till Kontoplan', action: () => executeNav('kontoplan') },
        { id: 'nav-rapporter', icon: '📈', label: 'Gå till Rapporter', action: () => { window.location.href = '/reports'; } },
        { id: 'nav-verktyg', icon: '🔧', label: 'Gå till Verktyg & Export', action: () => { window.location.href = '/tools'; } },
        { id: 'nav-installningar', icon: '⚙️', label: 'Inställningar', action: () => executeNav('installningar') },
        { id: 'action-ny-verifikation', icon: '✨', label: 'Ny Verifikation', action: () => executeAction('ny-verifikation') }
    ];

    function executeNav(view) {
        if (window.location.pathname === '/') {
            window.dispatchEvent(new CustomEvent('cmd-nav', { detail: view }));
        } else {
            // Om vi är på en annan sida, gå till index (SPA).
            // (Avancerat: vi skulle kunna skicka med view i sessionStorage för att öppna rätt flik vid laddning)
            sessionStorage.setItem('startup_view', view);
            window.location.href = '/';
        }
    }

    function executeAction(action) {
        if (window.location.pathname === '/') {
            window.dispatchEvent(new CustomEvent('cmd-action', { detail: action }));
        } else {
            sessionStorage.setItem('startup_action', action);
            window.location.href = '/';
        }
    }

    // 3. Render logik
    function renderResults(query = '') {
        const lowerQuery = query.toLowerCase();
        currentResults = commands.filter(c => c.label.toLowerCase().includes(lowerQuery));
        
        resultsList.innerHTML = '';
        
        if (currentResults.length === 0) {
            resultsList.innerHTML = '<li class="cmd-empty-state">Inga kommandon hittades...</li>';
            activeIndex = 0;
            return;
        }

        if (activeIndex >= currentResults.length) {
            activeIndex = 0;
        }

        currentResults.forEach((cmd, index) => {
            const li = document.createElement('li');
            li.className = 'cmd-item' + (index === activeIndex ? ' cmd-active' : '');
            li.innerHTML = `
                <span class="cmd-item-icon">${cmd.icon}</span>
                <span class="cmd-item-label">${cmd.label}</span>
                <span class="cmd-item-shortcut">Enter</span>
            `;
            
            li.addEventListener('mouseenter', () => {
                activeIndex = index;
                renderResults(input.value);
            });
            
            li.addEventListener('click', () => {
                closePalette();
                cmd.action();
            });
            
            resultsList.appendChild(li);
        });
        
        // Scrolla aktivt element i vy
        const activeEl = resultsList.querySelector('.cmd-active');
        if (activeEl) {
            activeEl.scrollIntoView({ block: 'nearest' });
        }
    }

    // 4. API för att öppna/stänga
    function openPalette() {
        isOpen = true;
        backdrop.classList.remove('cmd-hidden');
        input.value = '';
        activeIndex = 0;
        renderResults();
        // Liten delay för att transition ska hinna starta
        setTimeout(() => input.focus(), 50);
    }

    function closePalette() {
        isOpen = false;
        backdrop.classList.add('cmd-hidden');
        input.blur();
    }

    // 5. Event Listeners
    backdrop.addEventListener('click', (e) => {
        if (e.target === backdrop) closePalette();
    });

    input.addEventListener('input', (e) => {
        activeIndex = 0;
        renderResults(e.target.value);
    });

    input.addEventListener('keydown', (e) => {
        if (e.key === 'ArrowDown') {
            e.preventDefault();
            if (currentResults.length > 0) {
                activeIndex = (activeIndex + 1) % currentResults.length;
                renderResults(input.value);
            }
        } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            if (currentResults.length > 0) {
                activeIndex = (activeIndex - 1 + currentResults.length) % currentResults.length;
                renderResults(input.value);
            }
        } else if (e.key === 'Enter') {
            e.preventDefault();
            if (currentResults[activeIndex]) {
                const action = currentResults[activeIndex].action;
                closePalette();
                action();
            }
        } else if (e.key === 'Escape') {
            e.preventDefault();
            closePalette();
        }
    });

    // Global Keydown
    document.addEventListener('keydown', (e) => {
        // Kolla efter Ctrl+K eller Cmd+K
        if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
            e.preventDefault(); // Hindra webbläsarens standardsökning
            if (isOpen) {
                closePalette();
            } else {
                openPalette();
            }
        }
        
        if (e.key === 'Escape' && isOpen) {
            closePalette();
        }
    });
});
