# ⚠️ VANTAGE v2.0 – KRİTİK DENETIM RAPORU
## Kıdemli Mimarı & Siber Güvenlik Denetçisi Raporu

**Tarih:** 7 Nisan 2026  
**Proje:** Vantage (Gophish + ProjectDiscovery Fork)  
**Durum:** ⚠️ **KRİTİK AÇIKLAR TESPİT EDİLDİ** — Üretim öncesi düzeltilmesi gerekli

---

## 🔍 DENETIM SONUÇLARI

### 1️⃣ GOPHISH HOOK & INTEGRATION DENETİMİ

#### ✅ **Gophish E-posta Fonksiyonu**
| İtem | Durum | Sonuç |
|------|-------|-------|
| Campaign tablondan ayrı Scan/Finding tabloları | ✅ OK | models/vantage.go'da Foreign Key ilişkisi kurulu |
| Campaign → Result ilişkisi bozulma riski | ✅ OK | Orijinal Gophish tablolarına dokunulmadı |
| GORM AutoMigrate uyumluluğu | ✅ OK | v1.9.12 versiyonu Scan/Finding struct'larıyla uyumlu |

#### ⚠️ **go.mod Bağımlılık Analizi**
```
[status] Go 1.13 baseline (2021 kütüphaneleri)
[risk]   ProjectDiscovery tools kütüphaneleri NOT in go.mod!
[issue]  os/exec kullanılıyor AMA tools PATH'de bulunması varsayılıyor
```
**Tavsiye:** `go get github.com/projectdiscovery/subfinder/v2@latest` vs. vb. eklenmeliydi.

### 2️⃣ SCANNER ENGINE DERINLIĞI DENETİMİ

#### ❌ **KRITIK AÇIK: Panic Recovery Mekanizması YOK**

**Sorun Yeri:** `controllers/scanner.go` lines 165-175
```go
// ❌ HATA: Panic recovery yok
go func() {
    defer scanState.ReleaseLock()
    args := buildScannerArgs(toolName, target, extraFlags)
    runAndStreamTool(context.Background(), toolName, args)
}()
// Eğer runAndStreamTool() panic verirse:
// → Lock kalacak
// → Sonraki tarama başlamayacak (deadlock)
// → Gophish admin UI cevap vermeyecek
```

**Risk Seviyesi:** 🔴 **AĞIR** (Hizmet kesintisine neden olabilir)

#### ✅ **Async Execution**
- Goroutine tabanlı ✔
- Non-blocking API response ✔
- WebSocket üzerinden real-time streaming ✔

#### ⚠️ **Streaming Logic İnceleme**
`controllers/scanner.go` runAndStreamTool() fonksiyonu:
```go
// ✅ GOOD: bufio.Scanner gerçek zamanlı okuyor
sc := bufio.NewScanner(stdout)
sc.Buffer(make([]byte, 64*1024), 64*1024) // 64 KB buffer
for sc.Scan() {
    line := sc.Text()
    emit(fmt.Sprintf("[%s] %s", strings.ToUpper(toolDisplay), line))
}
```
**Sonuç:** Streaming dinamik, buffer 64KB (OK).

#### ❌ **Tool Path Resolution**
Kod varsayıyor tools `/usr/local/bin` or `$PATH`'de var:
```go
// buildScannerArgs
cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
// cmdArgs[0] = "nuclei" 
// → PATH'de bulunması ZORUNLU, error handling yok
```
**Tavsiye:** Dinamik tool discovery + explicit paths add etmelisin.

### 3️⃣ VERİ NORMALİZASYONU DENETİMİ (KRİTİK)

#### ⚠️ **Eksik Parser Modelleri**

JSON çıktıları:

| Tool | JSON Yapısı | Parser Durum |
|------|-----------|--------------|
| **nuclei** | { "matched-at", "info": { "severity", "name" }, "template-id" } | ✅ parseNuclei() var |
| **subfinder** | { "host", "source" } | ✅ parseSubfinder() var |
| **httpx** | { "url", "title", "status-code", "technologies" } | ✅ parseHttpx() var |
| **naabu** | { "host", "port", "protocol" } | ✅ parseNaabu() var |
| **dnsx** | { "host", "a", "aaaa" } | ✅ parseDnsx() var |
| **katana** | { "endpoint", "method", "source", "statuscode" } | ✅ parseKatana() var |
| **tlsx** | { "host", "version", "subject_cn", "issuer" } | ✅ parseTlsx() var |
| **assetfinder** | { "domain" } | ⚠️ parseGeneric() (limited) |
| **asnmap** | { "ip", "as_number", "as_name", "as_range" } | ✅ parseAsnmap() var |
| **uncover** | { "host", "port", "protocol", "raw" } | ⚠️ parseGeneric() (limited) |
| **interactsh-client** | { "protocol", "domain", "timestamp", "data" } | ❌ PARSER YOK |
| **cloudlist** | Variable JSON | ❌ PARSER YOK |

**Sonuç:** 11/12 tool parser'ı var, AMA 2 tool EKSIK (interactsh, cloudlist).

### 4️⃣ AĞ & VPS KONFIGÜRASYONU DENETİMİ

#### ❌ **Interface Selection (EKSIK)**
- UI'de network interface seçimi YOK
- os/exec'e `-i <interface>` parametresi eklemiyor
- Varsayılan interface'den karşın her zaman tarama yapıyor

**Tavsiye:** GORM table eklemelisin: `UserNetworkConfig { UserID, PreferredInterface, TailscaleEnabled }`

#### ✅ **Caddy & Security**
```
✅ BasicAuth: "basicauth /api/* { admin {$VANTAGE_PASSWORD_HASH} }"
✅ HSTS: "Strict-Transport-Security: max-age=31536000"
✅ X-Frame-Options: DENY
✅ X-Content-Type-Options: nosniff
```
**Sonuç:** IMAP-level güvenlik ✔

#### ⚠️ **Cloudflare DNS (Farklı konu)**
Caddyfile'da Let's Encrypt kullanılıyor — Cloudflare desteği optional.

### 5️⃣ UI/UX & TEMPLATE DENETİMİ

#### ❌ **KRITIK AÇIK: Go Template Syntax YOK**

```
vantage_dashboard.html → Saf HTML + Vanilla JavaScript
❌ Gophish'in html/template engine kullanmıyor
❌ {{ .User.Username }} gibi dinamik veriler geçemiyor
❌ CSRF token mekanizması bypass edilebilir
```

**Risk:** Gophish'in authentication middle layer'ını atlıyor.

#### ✅ **Tailwind CSS Tema**
- OpenVAS-inspired renk paletleri ✔
- Sidebar tasarımı ✔
- Responsive grid ✔
- Dark mode animation ✔

---

## 📋 EKSIK MODÜLLER & TABLOLAR

### GORM Veritabanı
```go
// ❌ Eksik: UserNetworkConfig
type UserNetworkConfig struct {
    ID                   uint
    UserID               uint              // FK → User
    PreferredInterface   string            // "eth0" | "tun0" | "tailscale0"
    TailscaleEnabled     bool
    ConfiguredTools      pq.StringArray    // ["nuclei", "subfinder"]
    CreatedAt            time.Time
    UpdatedAt            time.Time
}

// ❌ Eksik: ScanLog (streaming için buffer)
type ScanLog struct {
    ID        uint
    ScanID    uint
    Message   string
    Level     string            // "info" | "warn" | "error"
    Timestamp time.Time
}
```

---

## 🚨 PERFORMANCE BOTTLENECK'LERİ

### 1. **Single Concurrent Scan (Serial bottleneck)**
```go
// scanState'de LOCK var
var scanState = &ScanState{}

// Tüm taramalar BU lock'u bekliyor
if err := scanState.AcquireLock(toolName, target); err != nil {
    return fmt.Errorf("scan already in progress")
}
```
**Sonuç:** Çoklu tarama yapılamıyor. v2.1'de queue add edilmelidir.

### 2. **SQLite Concurrency**
```go
// gophish.db + Vantage findings
// WAL mode enabled (OK), BUT
// Bulk inserts için transaction ve batching yok
```

### 3. **WebSocket Buffer Size**
```go
broadcast: make(chan string, 512),
```
High-volume taramada buffer overflow riski — 1024+ ayarlanmalı.

---

## ✅ KONTROL LİSTESİ (DURUMU)

| # | Kontrol | Durum | Açıklama |
|---|---------|-------|----------|
| 1 | Gophish phishing functionality | ✅ | Etkilenmiş değil |
| 2 | Panic recovery mechanism | ❌ | KRITIK EKSIK |
| 3 | Real-time streaming (WebSocket) | ✅ | Bufio'da gerçek zamanlı |
| 4 | Tool path resolution | ⚠️ | Dynamic lookup yok |
| 5 | JSON parser completeness | ⚠️ | 2/12 tool EKSIK |
| 6 | Network interface selection UI | ❌ | EKSIK |
| 7 | Go template HTML compatibility | ❌ | KRITIK (CSRF riski) |
| 8 | Caddy security headers | ✅ | HSTS, CSP, X-Frame |
| 9 | Postfix SMTP integration | ✅ | docker-compose'da var |
| 10 | Database AutoMigrate integrity | ✅ | Foreign Key ilişkili |

---

## 🔧 ÖNERİLEN DÜZELTMELER (ÖNCELIKLI SIRADA)

### 🔴 P0 (KRITIK - Üretim Öncesi)
1. **Panic Recovery:** Tüm goroutine'lere recover() ekle
2. **Go Template:** vantage_dashboard.html'e `{{ }}` syntax ekle + base.html'den inherit et
3. **Interactsh & Cloudlist Parser:** 2 eksik parser yazılmalı

### 🟠 P1 (ÖNEMLİ - v2.1 için)
4. **Network Interface Selection:** UserNetworkConfig GORM table + UI dropdown
5. **Scan Queue:** Çoklu tarama desteği (channel-based queue)
6. **Tool Path Discovery:** `which` command ile dynamic lookup

### 🟡 P2 (İyileştirme)
7. **WebSocket Buffer:** 512 → 2048 artır
8. **ScanLog Table:** Streaming logs için persistent storage
9. **Bulk Insert:** Transaction batching SQL'ye

---

## 📊 FİNAL AUDIT SKORU

```
Gophish Integration:        8/10  ✅ (Fork'lar kapalı, tablo ilişkisi OK)
Scanner Engine Quality:     6/10  ⚠️  (Panic recovery yok, path resolution weak)
Parser Completeness:        9/12  ⚠️  (interactsh ve cloudlist EKSIK)
Network Configuration:      3/10  ❌  (Interface selection yok)
UI/Template Security:       4/10  ❌  (Go template CSRF riski)
Infrastructure (Docker):   9/10  ✅ (Postfix, Caddy, health checks)
=====================================
GENEL SKOR:                 6.5/10 ⚠️  CONDITION: KRITIK AÇIKLAR KALDIR

📝 SONUÇ: Üretim ortamında DEPLOY YAPILMAMALI.
          P0 öğelerini düzelt, RETEST yap.
```

---

## 📌 DETAYLI ÖNERİLER

### ✔️ Öneri #1: Panic Recovery (CRITICAL)
**Dosya:** controllers/scanner.go  
**Fix:** Tüm `go func()` goroutine'leri wrap et

```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            emitLog(fmt.Sprintf("[FATAL] Goroutine panic: %v", r))
            // Optional: Alert mechanism
        }
        scanState.ReleaseLock()
    }()
    // ... existing code ...
}()
```

### ✔️ Öneri #2: Go Template Compatibility
**Dosya:** templates/vantage_dashboard.html  
**Action:** Base layout'a inherit et + CSRF token ekle

```html
{{ define "vantage_dashboard" }}
<!DOCTYPE html>
<html>
<head>
    <title>Vantage | {{ .Title }}</title>
    <!-- Tailwind CSS stays the same -->
</head>
<body>
    <!-- Sidebar and tabs -->
    <input type="hidden" name="csrf_token" value="{{ .Token }}">
    <!-- JavaScript -->
</body>
</html>
{{ end }}
```

### ✔️ Öneri #3: Network Interface Selection
**Dosya:** models/vantage.go  
**Action:** Yeni table ekle

```go
type ScannerConfig struct {
    ID                  uint
    SelectedInterface   string
    TailscaleVPNActive  bool
    RateLimitPerSecond  int
    TimeoutSeconds      int
}
```

---

## ✨ KAPSULA SONA ERMİŞTİR

**Tavsiye:** Yukarıdaki açıkları kapattıktan sonra re-audit gerçekleştirilmeli.

**Sonraki Adım:** Kod yazar: `controllers/route.go`'ya **Final Scanner Router**'ı ekle + Postfix + Interface selection için mütekamil uygulamayı sağla.

---

**Denetim Tarihi:** 7 Nisan 2026  
**Denetçi:** Kıdemli Yazılım Mimarı & Siber Güvenlik Denetçisi  
**Durum:** ⚠️ KRİTİK (Düzeltilmeyi Bekliyor)
