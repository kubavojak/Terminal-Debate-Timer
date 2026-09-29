# DebTime CLI

Stopky pro akademické debaty v terminálu. Terminálová obdoba aplikace DebTime:
British Parliamentary, Karl Popper, World Schools, Řešitelská 2 × 2,
Sněmovní 2 × 2 a vlastní formát. Jeden statický soubor bez závislostí.

```
British Parliamentary                                                      3 / 9
╭──────────────────────────────────────────────────────────────────────────────╮
│ Vůdce opozice · OO 1                                                  ▶ běží │
│                                                                              │
│             ████████████            ████████████    ████████████             │
│             ████████████            ████████████    ████████████             │
│             ████            ████    ████            ████                     │
│             ████            ████    ████            ████                     │
│             ████████████            ████████████    ████████████             │
│             ████████████            ████████████    ████████████             │
│                     ████    ████            ████            ████             │
│                     ████    ████            ████            ████             │
│             ████████████            ████████████    ████████████             │
│             ████████████            ████████████    ████████████             │
│                                                                              │
│ [█████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░│░░░░░░░░]  POI povoleno │
╰──────────────────────────────────────────────────────────────────────────────╯
Další: Místopremiér · OG 2
␣ pauza  ←/→ krok  +/- čas  r reset  b zvonek  ? nápověda
```

## Instalace

```sh
# do ~/.local/bin
curl -fsSL https://raw.githubusercontent.com/kubavojak/Terminal-Debate-Timer/main/install.sh | sh

# nebo do /usr/local/bin (sudo)
curl -fsSL https://raw.githubusercontent.com/kubavojak/Terminal-Debate-Timer/main/install.sh | sh -s -- --system
```

Po instalaci stačí v terminálu napsat `debtime`. Když příkaz není nalezen,
otevřete nový terminál (`~/.local/bin` se do PATH přidá až po novém přihlášení).

### Balíček .deb (Ubuntu, Debian)

Balíček je potřeba nejdřív stáhnout a pak nainstalovat ze stažené složky
(`./` na začátku je nutné):

```sh
curl -fLO https://github.com/kubavojak/Terminal-Debate-Timer/releases/download/v0.1.0/debtime_0.1.0_amd64.deb
sudo apt install ./debtime_0.1.0_amd64.deb
```

Pro Raspberry Pi nahraďte `amd64` za `arm64` (64bitový systém) nebo `armv7`
(32bitový). Nejnovější verze je vždy na stránce
[Releases](https://github.com/kubavojak/Terminal-Debate-Timer/releases).

### Balíček .rpm (Fedora, openSUSE)

```sh
curl -fLO https://github.com/kubavojak/Terminal-Debate-Timer/releases/download/v0.1.0/debtime_0.1.0_amd64.rpm
sudo dnf install ./debtime_0.1.0_amd64.rpm
```

### Samotný soubor

```sh
curl -fL https://github.com/kubavojak/Terminal-Debate-Timer/releases/download/v0.1.0/debtime_0.1.0_linux_amd64.tar.gz | tar -xz debtime
./debtime
```

## Použití

```
debtime [flags]
  --format bp|kp|wsdc|resitelska|snemovni|custom
  --speakers N --minutes M        (pro custom)
  --consultation                  (porada u Řešitelské)
  --lang cs|en                    (výchozí podle $LANG, jinak cs)
  --sound auto|bell|off
  --no-restore                    (ignorovat uložený stav)
  --high-contrast
  --list-formats                  (vypíše formáty a skončí)
  --version
  --debug                         (log do ~/.local/state/debtime/debug.log)
debtime [flags] plan <format>     (vypíše pořadí a délky kroků bez TUI)
```

Při prvním spuštění se otevře výběr formátu. Stav stopek se průběžně ukládá,
takže po zavření a novém spuštění debata pokračuje (běžící stopky započítají
i čas, kdy byla aplikace zavřená).

### Klávesy

| Klávesa | Akce |
|---|---|
| `mezerník` | start / pauza / pokračovat |
| `←` `→` / `h` `l` | předchozí / další krok |
| `n` / `Enter` | další část (u dotazování „ukončit dotazování") |
| `+` / `-` | ±10 s uplynulého času |
| `]` / `[` | ±1 min uplynulého času |
| `r` | reset aktuálního kroku |
| `R` | restart celé debaty (s potvrzením) |
| `b` | ruční zvonek |
| `1` / `2` | start/pauza přípravného fondu týmu (Karl Popper) |
| `↑` / `↓` | počet otázek ±1 (Sněmovní) |
| `u` | počítání nahoru / dolů |
| `f` | výběr formátu |
| `L` | přehled kroků (Enter = skok na krok) |
| `p` | přípravné fondy |
| `s` | nastavení |
| `?` | nápověda |
| `q` | konec (s potvrzením, když něco běží); `Ctrl+C` vždy hned |

### Barvy

Chráněný čas žlutě, POI povoleno a volný čas zeleně, přesčas červeně
(bliká), příprava modře, dotazování fialově. `NO_COLOR` vypne barvy,
`--high-contrast` přepne na výrazné barvy a tučné písmo.

## Zvuk

1. Přibalené zvuky (`single`, `double`, `continuous`) přehrané přes
   `pw-play`, `paplay` nebo `aplay` (první nalezený). Když přehrávač selže
   (např. přes SSH bez zvukového serveru), přepne se na zvonek.
2. Terminálový zvonek (BEL): jednou, dvakrát po 200 ms, souvisle po 300 ms
   5 sekund. Funguje i přes SSH, ale v terminálu může být vypnutý.
3. Vždy vizuální signál: rámeček stopek bliká inverzně.

Souvislé pípání se zastaví při pauze, resetu nebo změně kroku.

## Soubory

- `$XDG_CONFIG_HOME/debtime/settings.json` (výchozí `~/.config/debtime/`)
- `$XDG_STATE_HOME/debtime/timer_state.json` (výchozí `~/.local/state/debtime/`)
- `$XDG_CACHE_HOME/debtime/sounds/` rozbalené zvuky

Poškozený soubor se ignoruje a použijí se výchozí hodnoty.

## Vývoj

```sh
go test ./...                      # testy
go test ./internal/tui -update     # přegenerovat snapshoty obrazovky
go generate ./assets               # přegenerovat WAV soubory
CGO_ENABLED=0 go build -o debtime ./cmd/debtime
```

Struktura: `internal/format` (formáty jako data), `internal/engine` (stav
stopek, signály), `internal/clock` (čas s ochranou proti uspání),
`internal/tui` (Bubble Tea), `internal/sound`, `internal/store`,
`internal/i18n`.

## Vydání

Tag `v*` spustí GitHub Actions a GoReleaser: binárky pro linux/amd64,
linux/arm64, linux/armv7 a macOS, archivy `.tar.gz`, `checksums.txt`,
balíčky `.deb` a `.rpm`.
