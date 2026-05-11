# Local Ledger

Ett lokalt ("On-Premise") mikrobokföringssystem designat för småföretag. Byggt med Go, SQLite och Vanilla HTML/JS för att garantera extrem snabbhet, dataägandeskap och zero-config distribution.

## Teknisk Stack
- **Backend:** Go (Golang)
- **Databas:** SQLite (Lokal fil)
- **Frontend:** Vanilla HTML, CSS, JS (Inga byggsteg)

## Arkitektur
Systemet är uppbyggt kring en strikt domändriven design där Bokföringslagens (BFL) krav på spårbarhet (immutability) och dubbel bokföring (Debet=Kredit) enforcing direkt i `internal/ledger/`-lagret och säkras i SQLite.

## Starta systemet
För att köra systemet lokalt:
```bash
go run cmd/localledger/main.go
```
Applikationen startar på `http://127.0.0.1:8080`.

## Dokumentation
Läs `ARCHITECTURE.md` och PRD-dokumenten i `docs/`-mappen för full förståelse av systemets legala och tekniska krav.
