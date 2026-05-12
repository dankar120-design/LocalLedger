# 🤖 AGENT INSTRUCTIONS: FRONTEND

Här ligger klientkoden. Zero-build-step filosofi.

1. **Teknikstack:** Vanilla HTML, CSS och JS. Vi undviker React/Vue/NPM-byggsteg för att hålla projektet "On-Premise" och extremt lättviktigt.
2. **Estetik:** Systemet ska se "dyrt" och modernt ut. Återanvänd designmönster från KärnFaktura men skala upp dem för ett affärssystem.
3. **Ikonografi:** ALLTID inline SVG. Vi har infört ett strikt *emoji-förbud* i gränssnittet. Använd `.nav-icon` (20x20) eller `.icon-xl` (3rem) för konsekvent rendering oavsett OS.
4. **API-kommunikation:** Använd native `fetch()` för att kommunicera med Go-API:et.
5. **Modulär kod:** Bryt upp JS i ES6-moduler om filerna blir för stora, men håll det enkelt.
