# Fluxboard: Production hazırlığı ve ürün geliştirme araştırması

**Tarih:** 9 Ekim 2026

**Hedef müşteri:** Farklı sektörlerdeki küçük ve orta ölçekli ekipler; kullanıcının bu araştırma sırasında belirttiği hedef.

**İncelenen kod:** `391f83374e671622151bacc378e013e544c58e28`, başlangıç dalı `feat/ui-redesign`.

**Araştırma dalı:** `docs/production-product-research`; yayın dalı: `docs/publish-production-report`.

**Karar niteliği:** Araştırma ve öneri; kabul edilmiş ADR veya uygulama talimatı değildir.

## 1. Karar özeti

Fluxboard'un ücretli, genel kullanıma açık production çıkışı için önce güvenilirlik ve gerçek hizmet entegrasyonları tamamlanmalı. Bugünkü kaynak kod, görev yönetimi bakımından geniş bir temel içeriyor; ödeme ve AI tarafında ise kullanıcıya sunulacak gerçek servis henüz tamamlanmamış. README'deki “all phases complete” ifadesi ticari çıkış onayı olarak kullanılmamalı.

**Tek ana öneri:** Fluxboard'u, küçük ve orta ölçekli ekiplerin **talep → iş planı → sorumlu → onay → sonuç** akışını hızlı kurduğu, günlük işleri ve müşteri işlerini tek yerde takip ettiği bir ürün olarak geliştirmek. Bunu güvenilir görev yönetimi, hazır süreçler, tekrarlayan işler, anlaşılır fiyatlandırma ve kaynak gösteren raporlarla desteklemek.

Bu yön, her sektöre ayrı bir ürün yapmak anlamına gelmez. Ortak bir görev/süreç çekirdeği üzerinde sektörlere göre şablonlar sunulmalı. İlk pilotta pazarlama, operasyon ve müşteri hizmeti veren ekipler gibi birkaç kullanım biçimi seçilmeli; sağlık, finans gibi özel mevzuat yükü olan alanlara uygunluk kanıtı olmadan satış vaadi verilmemeli.

**Alternatif:** Pilotlar esas olarak yazılım ekiplerinden oluşursa Linear benzeri issue/cycle/GitHub odağına daralmak. Kullanıcının belirttiği hedef için bu, başlangıç önerisi değildir.

İlk ücretli sürümün öncelik sırası:

1. Gerçek ödeme, tutarlı plan kataloğu, güvenli production ayarları, gerçek test ve geri yükleme kanıtı.
2. İlk gün kullanım: veri içe aktarma, süreç şablonu, ekip daveti ve ilk görevi bitirme.
3. Tekrarlayan işler, talep toplama ve onay akışları.
4. Google/Microsoft takvimleri, e-posta ve hedef müşterilerin kullandığı iletişim araçları.
5. Açıklanabilir kapasite ve durum raporları; ardından kontrollü gerçek AI.

Yeni bir görünüm eklemek, bir ödeme hatasını veya sessiz veri kaybını telafi etmez. Çıkış kararı özellik sayısına göre değil, aşağıdaki kabul kanıtlarına göre verilmeli.

## 2. Araştırmanın yöntemi ve sınırları

Çalışmada README, ürün gereksinimleri, mimari/altyapı/test dokümanları, build tracker, API router, usecase servisleri, SQL sorguları ve migrations, frontend API istemcileri, seçili sayfa/bileşenler, CI ve deploy dosyaları incelendi. Ayrıca rakiplerin resmi ürün/fiyat sayfaları ve ilgili standart/servis dokümanları web üzerinden okundu.

**Kanıt seviyeleri:**

| İşaret | Anlam |
|---|---|
| Kod kanıtı | İncelenen kaynakta doğrudan görülen davranış veya yapı |
| Statik risk | Kaynak akışından çıkarılan sonuç; runtime hata enjeksiyonu veya saldırı testi yapılmadı |
| Dış kaynak | Araştırma tarihinde resmi sayfada görülen bilgi |
| Öneri/hipotez | Fluxboard için önerilen tasarım, hedef veya pazar varsayımı; ölçülmüş mevcut sonuç değildir |

Bu çalışma bir penetration test, eksiksiz güvenlik taraması, kullanıcı görüşmesi veya çalışan ürünün görsel UX denetimi değildir. Browser üzerinden uygulama çalıştırılmadı; backend/frontend testleri ve load test yeniden yürütülmedi. Canlı altyapı, gerçek ödeme hesabı, uzak GitHub CI sonucu ve branch protection ayarları doğrulanmadı. Bu nedenle “production testleri geçti”, “kullanıcılar bunu istiyor” veya “rakiplerden daha hızlı” sonucu çıkarılamaz.

Yerel snapshot'ta **62 frontend `page.tsx` dosyası**, **45 Go `_test.go` dosyası**, **0032'ye kadar migration** ve sunulan OpenAPI JSON'unda **3 path** görüldü. Bunlar tamlık veya kalite puanı değildir. Özellikle dosya sayısı, akışların çalıştığını kanıtlamaz.

Önceki çalışma notları yalnızca doküman/kod ayrımı ve doğrulama yaklaşımı için kullanıldı. Buradaki proje bulguları mevcut checkout üzerinden yeniden kontrol edildi.

## 3. Projede zaten bulunanlar

“Ekle” önerisi vermeden önce mevcut temel ayrılmalı:

| Alan | Kaynakta bulunan kapsam | Ticari ürün açısından değerlendirme |
|---|---|---|
| Kimlik ve oturum | E-posta/parola, doğrulama/reset, JWT ve refresh rotation, Google OAuth, TOTP, oturum yönetimi | Temel var; production konfigürasyonu ve gerçek e-posta/OAuth akışları kanıtlanmalı |
| Organizasyon ve yetkiler | Çoklu organizasyon, davet, OWNER/ADMIN/MEMBER/GUEST, proje rolleri, PostgreSQL RLS | Korunmalı; tenant izolasyonu ile aynı tenant içindeki private proje yetkisi ayrı kontrol edilmeli |
| Görev yönetimi | Kanban/list, sıralama, öncelik, sorumlu, tarihler, etiket, alt görev, yorum, ekler, arama, bulk, trash | İlk ücretli ürünün omurgası olabilir |
| Zaman ve planlama | Zaman kaydı/timer, sprint lifecycle, task dependency ve cycle detection, calendar, timeline | Sıfırdan eklemek yerine tutarlılık ve kullanıcı değeri geliştirilmelidir |
| Özelleştirme | Proje custom field tanımları/değerleri, frontend proje şablonları | Tekrar kullanılan süreç şablonuna dönüştürülmeli |
| Talepler | Token ile public form, submission'dan görev oluşturma, form token rotation | Talep takibi ve müşteri geri bildirimi zinciri eksik |
| Otomasyon | `task.created`, `task.moved`, `task.assigned`; assign/priority/move/label aksiyonları | Basit temel; yürütme geçmişi, hatanın görünürlüğü ve zaman tetikleyicileri gerekli |
| İşbirliği | SSE, bildirim merkezi, e-posta işleri, preference yüzeyleri | Gerçek teslimat ve bağlantı toparlanması test edilmeli |
| Billing | Plan/entitlement, abonelik mirror, webhook idempotency, usage jobs, invoice/portal ekranları | **Gerçek Stripe gateway yok; ücretli çıkışa hazır değil** |
| AI | Parse/plan/chat/digest/risk, run/risk ledger, quota/feature flags, apply | **Provider mock; gerçek AI hizmeti yok** |
| Yönetim/operasyon | Admin, audit, API key, impersonation, metrics/Grafana, health/readiness | Altyapı temeli var; alarm, backup, release ve incident kanıtı eksik |

Kaynaklar: [router](../../backend/internal/interface/http/router.go), [sprint servisleri](../../backend/internal/usecase/projectuc/service_sprint.go), [zaman takibi](../../backend/internal/usecase/taskuc/service_time.go), [özel alanlar](../../backend/internal/usecase/projectuc/service_fields.go), [formlar](../../backend/internal/usecase/projectuc/service_form.go), [otomasyon](../../backend/internal/usecase/automationuc/service.go), [frontend şablonlar](../../web/lib/board/templates.ts).

### Doküman ile kodun ayrıldığı noktalar

- `docs/00-PROJECT-OVERVIEW.md` sprint, zaman takibi ve Gantt'ı v1 non-goal sayıyor; kodda sprint, timer ve temel timeline bulunuyor. Bunlar aynı olgunlukta tam ürünler değildir, ama yok da değildir.
- README/build tracker hâlâ 55 route ve org-logo endpoint follow-up'ından söz ediyor; mevcut checkout'ta 62 route ve logo upload URL servisi/router endpoint'i bulunuyor.
- `docs/10-INFRA-DEVOPS.md`, integration testte MinIO servisinden söz ediyor; CI açıkça MinIO çalıştırmadığını belirtiyor.
- Plan fiyat ve limitleri overview, marketing, frontend billing referansı ve backend seed arasında ayrılıyor.
- FR-AI-007 JSON-RPC MCP tarif ediyor; mevcut handler özel `{tool, params}` HTTP dispatch formatı kullanıyor.

**İlerleme kuralı:** Uygulama başlamadan kapsam ve değişen sözleşmeler ADR ile netleştirilmeli. Bu rapor mevcut ADR'leri kendiliğinden değiştirmez. “Tamamlandı” yerine her modül için `implemented / integration verified / staging verified / production enabled` ayrımı kullanılmalı.

## 4. Production çıkışını engelleyen veya koşula bağlayan bulgular

Öncelik tanımı: **BLOCKER** ilgili çıkış türünü engeller; **IMPORTANT** pilot veya sonraki sürümde ciddi kullanım/operasyon riski oluşturur; **MINOR** temel kabulü engellemeyen düzeltmedir. “Kanıt yok” ifadesi “canlıda kesin yok” anlamına gelmez.

### B01 — Gerçek ödeme entegrasyonu tamamlanmamış — BLOCKER: ücretli çıkış

**Kod kanıtı:** `backend/internal/infrastructure/stripe/client.go:24` içindeki constructor `live` için hata döndürüyor. Stub checkout/portal URL üretir, abonelik değişimleri ve usage push no-op'tur. `backend/cmd/stripeseed/main.go` da live seeding'i reddeder.

**Sonuç:** Mevcut billing ekranları gerçek tahsilat, gerçek proration veya Stripe mutabakatı kanıtı değildir. Sahte webhook testi gerçek Stripe signature/payload testinin yerine geçmez.

**İlerleme:** Mevcut gateway portunu koruyarak gerçek adapter yaz; gerçek test hesabıyla checkout, abonelik güncelleme, iptal/resume, ödeme başarısızlığı, portal, invoice mirror ve reconciliation zincirini tamamla. API sürümünü ve SDK sürümünü uygulanacağı tarihte resmi uyumluluk bilgisiyle sabitle; eski yorumdaki `stripe-go/v78` ifadesini güncel tercih kabul etme.

Legacy usage-records tasarımını yeni adapter'a taşımadan Billing Meters sözleşmesini değerlendir. Stripe, legacy usage API'lerinin `2025-03-31.basil` sürümünde kaldırıldığını belgeler. Depolama gauge'ı, API event toplamı ve ücretlendirilen koltuk aynı aggregation semantiği değildir. [Stripe migration rehberi](https://docs.stripe.com/billing/subscriptions/usage-based-legacy/migration-guide).

**Kabul:** Gerçek Stripe test ortamında aynı event replay'i tek yan etki yaratır; ters sıradaki event durumu geriye götürmez; başarısız ödeme entitlement'ı doğru değiştirir; invoice toplamı bağımsız hesapla eşleşir; kayıp webhook reconciliation ile toparlanır. Public ücretli açılış öncesinde kontrollü gerçek tahsilat/iptal kanıtı alınır. [Webhook davranışı](https://docs.stripe.com/webhooks).

**Hesap uygunluğu koşulu:** İşletmenin ülkesi ve tüzel kişiliği bu araştırmada bilinmiyor. Stripe'ın mevcut desteklenen işletme ülkeleri listesinde Türkiye görünmüyor; bu, Türkiye'deki müşterilerin kartlarıyla ödeme yapabilmesiyle aynı konu değildir. Türkiye tüzel kişiliğiyle satış yapılacaksa sağlayıcı hesabı uygunluğu ödeme geliştirmesinden önce netleştirilmeli. [Stripe ülke listesi](https://stripe.com/global). Tek anlamlı alternatif, uygunluğu doğrulanmış bir Merchant of Record üzerinden satış; örneğin Paddle'ın Türkiye sayfası ve supplier ülke politikası araştırılabilir, ancak hesap kabulü garanti değildir. Bu değişiklik projenin “Stripe only” kararını değiştireceği için ADR gerekir. [Paddle Türkiye](https://www.paddle.com/billing/turkey), [supplier ülkeleri](https://www.paddle.com/help/start/intro-to-paddle/which-countries-are-supported-by-paddle).

### B02 — Fiyat ve plan sözleşmesi tutarsız — BLOCKER: ücretli çıkış

**Kod kanıtı:**

| Kaynak | Free | Pro | Business |
|---|---|---|---|
| Overview | 3 üye, 2 proje, 100 MB | $12/seat, 25 üye, 20 proje, 10 GB | $24/seat, sınırsız üye/proje, 100 GB |
| Marketing constants | 3 üye, 1 proje, 100 MB | 1200 cent; 15 üye, 20 proje, 10 GB | 4900 cent; 100 üye, sınırsız proje, 100 GB |
| Frontend billing referansı | 5 üye, 3 proje, 2 GiB | 25 üye, 50 proje, 50 GiB | sınırsız üye/proje, 500 GiB |
| Backend stub seed | 5 üye, 3 proje, 2 GiB | 1200 cent; 25 üye, 50 proje, 50 GiB | 4900 cent; sınırsız üye/proje, 500 GiB |

Ayrıca `billinguc/service.go:214` ve `:227`, proration/update çağrılarına koltuk sayısını `1` gönderiyor; seat mirror için TODO bulunuyor.

**İlerleme:** Tek versioned plan kataloğu ve server-authoritative fiyat/entitlement endpoint'i; marketing ve app aynı kaynaktan okumalı. Aylık/yıllık dönem, fiyatın organizasyon başına mı koltuk başına mı olduğu, guest'in ücretlendirilebilirliği, vergi ve overage davranışı açık sözleşme olmalı. Legacy müşterinin plan sürümü yeni kataloğa sessizce taşınmamalı.

**Kabul:** Fiyat ekranı, checkout, invoice ve entitlement fixture'ı aynı plan sürümüne bağlı; 1/5/25/26 üyeli senaryolarda toplam ve haklar beklenen değerde; plan değişimi mevcut quantity'yi yanlışlıkla 1'e indirmiyor. [Stripe quantity modeli](https://docs.stripe.com/billing/subscriptions/quantities).

### B03 — Integration CI gerçek DB testini atlayabilir — BLOCKER: çıkış kabul kanıtı

**Kod kanıtı:** `webhook_repo_integration_test.go:34-36`, `TEST_DATABASE_URL` yoksa `t.Skip` yapıyor. `.github/workflows/ci.yml:74` integration-tag komutunu çalıştırıyor fakat bu değişkeni vermiyor; workflow'da role/migration kurulum adımları da görünmüyor. Integration tag taşıyan test dosyası taramasında yalnızca bu dosya bulundu.

**Sonuç:** Workflow'da PostgreSQL servisinin bulunması testin DB'ye bağlandığını kanıtlamaz. Yeşil process exit, skip edilmiş integration test ile elde edilebilir. Uzak CI bu çalışmada okunmadı; burada tespit edilen sorun workflow/test sözleşmesidir.

**İlerleme:** CI role kurulumunu/migrations'ı yürütmeli; DSN'leri doğru role bağlamalı; gerekli integration testlerde eksik konfigürasyon skip yerine fail olmalı. Fixture kurulumu için privileged bağlantı ile uygulama RLS test bağlantısı ayrılmalı. MinIO/S3 uyumlu test storage eklenmeli. Browser E2E staging/CI gate'e alınmalı.

**Kabul:** CI logunda suite gerçekten koşar; iki tenant ve aynı tenant içinde private proje matrisi DB'nin `fluxboard_app` rolüyle geçer; attachment upload→confirm→download→delete gerçek object storage ile geçer. Web ve security job'larının required status olup olmadığı uzak repo ayarlarından ayrıca doğrulanır. Dosyadaki yorum branch protection kanıtı değildir.

### B04 — Outbox kuyruğa aktarım başarısızlığında kayıp iş riski — BLOCKER: güvenilir teslimat

**Kod kanıtı:** `queries/outbox.sql:13`, claim sırasında `drained_at=now()` yazar. `jobs/billing.go:136`, enqueue başarısızlığında kaydı loglayıp devam eder. Sonraki claim yalnızca `drained_at IS NULL` satırlarını seçer.

**Statik sonuç:** Claim ile enqueue arasındaki Redis kesintisi/process crash, normal drainer akışının artık almayacağı bir kayıt bırakabilir. TaskID deduplication, hiç enqueue edilmeyen işin kaybını çözmez.

**İlerleme:** Kalıcı outbox durumunu `pending/claimed/enqueued`, lease expiry, attempt/error bilgisi ve retry ile modelle; kuyruğun kabulünden sonra enqueued işaretle. Enqueue başarılı, DB ack başarısız senaryosunda duplicate kabul edip consumer side effect'ini idempotent yap. PostgreSQL ile Redis arasında atomik transaction varmış gibi tasarlama. [Transactional outbox ilkeleri](https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/transactional-outbox.html).

**Kabul:** Claim sonrası kill, Redis down, enqueue sonrası ack öncesi kill ve worker retry testlerinde iş kaybolmaz; desteklenen yan etkiler tekrar altında idempotent; sorun admin ekranında görülebilir ve replay edilebilir. E-posta sağlayıcısının idempotency garantisi yoksa “exactly once email” sözü verilmez.

### B05 — Production kritik ayarları eksikken sessiz fallback — BLOCKER: public deployment

**Kod kanıtı:** `cmd/api/main.go:497` boş JWT key için ephemeral key üretir; yorum “dev only” olsa da fonksiyonda production koşulu yok. Boş TOTP key 2FA'yı kapatır, object-store bağlantı hatası attachment'ı kapatarak boot'a devam eder. Mailer, SMTP yoksa mesajın alıcısını/konusunu/gövdesini loglar. `AI_PROVIDER` tanınmıyorsa mock'a döner.

**Sonuç:** Production adıyla başlayan süreç, kritik özelliği dev dışı ortamda kapalı veya sahte halde çalıştırabilir. Ephemeral key restart/replica davranışını bozar; SMTP fallback'inin production'a taşınması işlem linklerini loglara düşürebilir.

**İlerleme:** Production validation profile oluştur: kalıcı JWT/TOTP keys, seçilen gerçek billing modu, zorunlu e-posta ve storage, beklenen origin/redirect, secret erişimi ve TLS yapılandırması. Opsiyonel yüzeyler açıkça disabled olmalı, ücretli hizmetin sessiz fallback'i olmamalı. API/worker owner pool kullanımını inventory et; migration credential ile sürekli çalışan privileged servis erişimini minimum yetkili ayrı role ayrıştırmayı değerlendir.

**Kabul:** Kritik config eksikse startup nonzero exit; stub/mock kullanımı prod hizmet vaadi altında reddedilir veya ilgili ürün yüzeyi kapalıdır; iki API replica ortak imza doğrulaması yapar; restart oturumları sebepsiz bozmaz; işlem linkleri loglarda yoktur. Anahtar rotasyonu ve geri dönüş planı belgelenir.

### B06 — AI yüzeyinde private proje yetkisi kontrol boşluğu — BLOCKER: AI açık çıkış

**Statik kanıt zinciri:** Router AI endpoint'lerini org `read` kapısından geçirir. `aiuc.Digest`, `ScanRisks`, `ListRisks` proje repository `Get` ve task `ListByProject` çağırır; burada `projectuc.access` benzeri kullanıcı/proje rol kontrolü görünmüyor. `ProjectRepo.Get` sorgusu org+id ile seçer; migration RLS koşulu yalnızca tenant'tır. `DismissRisk` de task repository read'ini görünürlük kanıtı sayar.

**Olası etki:** Aynı organizasyonda bulunan ama private projeye üye olmayan bir kullanıcının AI üzerinden proje istatistiği/risk bilgisi okuyabilmesi. Bu, farklı tenant'a veri sızdığı iddiası değildir. Çalışan API üzerinde PoC yapılmadı; statik kontrol eksikliğidir.

**İlerleme:** AI prompt/context/read/write yollarında aynı project authorization policy'sini kullan. Org RLS'nin kullanıcı düzeyi private visibility'yi sağlamadığını kabul et. API key service identity, guest ve impersonation için ayrı negatif vaka ekle. [PostgreSQL RLS kapsamı](https://www.postgresql.org/docs/16/ddl-rowsecurity.html).

**Kabul:** Aynı tenant içinde MEMBER/GUEST olmayan private proje erişiminde AI digest/risk/MCP/context çıktısı bilgi vermez; authorized LEAD/CONTRIBUTOR/VIEWER pozitif yolları çalışır. AI yüzeyi kapalıysa bu bulgu AI açılış kapısında kalır; temel beta tüm AI yüzeyleri erişilemezken ayrıca değerlendirilebilir.

### B07 — Backup, restore, release ve alarm kanıtı eksik — BLOCKER: müşteri verisiyle public çıkış

**Kod/doküman kanıtı:** Restore runbook otomatik backup provision edilmediğini açıkça söylüyor. Deploy dosyaları yerel reference Compose; dış network/TLS, production IaC/release pipeline görünmüyor. Prometheus config scrape içeriyor fakat `rule_files` veya `alerting` yok. `/readyz` DB+Redis ping kontrol ediyor; migration version, storage ve ödeme uygunluğu kontrolü yapmıyor.

Restore runbook'un custom-format dump'ı PowerShell `Get-Content -Raw` ile metin olarak `pg_restore` stdin'e taşıması da binary dosya için güvenilir prosedür kabul edilmemeli. Dosya byte olarak container/recovery ortamına taşınıp dosya yolu üzerinden restore edilmeli; exact komut seçilen ortamda doğrulanmalı.

**İlerleme:** Managed PostgreSQL + PITR; uygulamadan bağımsız erişim sınırına sahip backup; object storage versioning/retention; DB+blob recovery manifest'i; staging ve versioned release artefact'ları; migration expand/contract; rollback; dış uptime probe; çalışan alarm receiver.

Başlangıç önerisi: tek bölge, Go modular monolith ve Next web; minimum güvenilir servis seti. Kubernetes veya microservice dönüşümü çıkış koşulu değildir. Managed DB/storage, küçük ekibin recovery yükünü düşürür. Türkiye barındırma vaadi varsa sağlayıcı/bölge buna göre seçilmeli veya vaat düzeltilmelidir.

**Kabul hedefi — öneri:** DB RPO ≤15 dakika, servis RTO ≤4 saat; gerçek dump/PITR ile temiz ortama restore, role/RLS ve ek dosya erişimi testi, tarihli sonuç. Bu değerler mevcut başarı veya SLA değil; hedeflerin maliyeti ve müşteri ihtiyacıyla onaylanması gerekir. PITR DB'yi geri getirir; object storage ve config ayrıca toparlanır. [PostgreSQL recovery dokümanı](https://www.postgresql.org/docs/16/continuous-archiving.html).

### B08 — Destek dışı frontend sürümü — BLOCKER: public sürüm bakım politikası

**Kod kanıtı:** `web/package.json`, Next.js `14.2.35` kullanıyor. Resmi destek politikası bugün 14.x'i unsupported, 16.x'i Active LTS, 15.x'i Maintenance LTS listeliyor. Bu tespit tek başına belirli bir CVE'nin uygulamada exploitable olduğunu söylemez. [Next.js destek politikası](https://nextjs.org/support-policy).

**İlerleme:** Aktif desteklenen stabil sürüme kontrollü migration; App Router params, auth redirects, build/lint/test araçları ve React uyumluluğu birlikte kontrol edilmeli. Major yükseltmede mevcut `next lint` script'inin çalışacağı varsayılmamalı.

**Kabul:** Desteklenen patch sürümü pinned; frozen dependency install; auth/billing/E2E gates; dependency ve container scan; rollback build'i hazır. Bu rapor uygulama değişikliği yapmaz.

### B09 — Hukuki/operasyonel vaatler gerçek işletmeye bağlanmamış — BLOCKER: ticari public site

**Kod kanıtı:** Privacy/DPA/terms sayfalarında `privacy@fluxboard.local`, Türkiye'de hosting, subprocessor sözleşmeleri ve güvenlik vaatleri bulunuyor. Sayfaların bulunması, bu koşulların sağlandığını kanıtlamaz. Operasyon entity'si, gerçek iletişim domain'i ve provider/bölge envanteri bu çalışmada doğrulanmadı.

**İlerleme:** Gerçek veri sorumlusu/işleyen rolleri, sağlayıcı listesi, bölgeler, AI veri akışı ve retention, destek/ihlal iletişimi, export/deletion prosedürü ve ticari sözleşme işletmenin gerçek yapısına göre düzenlenmeli. Türkiye dışı AI/e-posta/ödeme kullanımında aktarım mekanizması ayrıca değerlendirilmeli; “onay kutusu var” yeterli kabul edilmemeli. KVKK rehberi ve uygun profesyonel inceleme bu kararın girdileri olmalı. [KVKK aktarım rehberi](https://www.kvkk.gov.tr/Icerik/8143/Kisisel-Verilerin-Yurt-Disina-Aktarilmasi-Rehberi), [standart sözleşme açıklaması](https://www.kvkk.gov.tr/Icerik/8170/Yurt-Disina-Kisisel-Veri-Aktariminda-Kullanilacak-Standart-Sozlesmelerde-Dikkat-Edilmesi-Gereken-Hususlara-Iliskin-Kamuoyu-Duyurusu).

**Kabul:** Site metni deploy edilen bölge/sağlayıcı/hizmetle uyuşur; iletişim adresi çalışır; erişim/export/silme talebi test edilir; backup'taki silinmiş verinin retention davranışı açıklanır; AI provider varsa alt işleyen listesine ve veri akışına dahil edilir.

### I01 — Analytics doğru sonuç üretmiyor — IMPORTANT; rapor satışı için BLOCKER

`StatsRepo.ComputeDay` yalnızca created count ve current column snapshot hesaplıyor; kaynak yorumunda `CompletedCount=0`, `AvgCycleSeconds=nil` açıkça belirtilmiş. `analyticsuc` bu rollup alanlarını okuyor. Tamamlanma/cycle-time ekranını gerçek performans kanıtı saymak yanlış olur.

İlerleme: Kolon adına veya son sıraya güvenmeden workflow state category tanımla; task activity üzerinden tamamlanma, tekrar açılma ve cycle-time kurallarını belirle; günlük snapshot'ın hangi zamana ait olduğunu göster; geçmişi tekrar hesaplama/backfill sözleşmesi yaz. Günlük ortalamaların basit ortalaması yerine örnek sayısıyla ağırlıklandırılan ölçümü değerlendir.

Kabul: 10 görevli bağımsız fixture'da expected completed/cycle-time değerleri tutar; done→reopen→done ve kolon reorder metrikleri bozmaz; yeterli veri yoksa `N/A`, sahte sıfır gösterilmez.

### I02 — AI plan apply atomiklik/idempotency sınırı — IMPORTANT; AI write açılışı için BLOCKER

`aiuc/apply.go:78` görevleri sırayla oluşturur, ilk hatada best-effort trash yapar; compensation hatası loglanır. Redis key önce okunur, writes sonrası set edilir. Run ledger unique key'i, daha önce oluşan görevleri transaction içinde korumaz. `newRun` ledger hatasını kullanıcıya taşımadan run ID döndürebilir.

Sonuç: Bu tasarım DB transaction all-or-none değildir. Concurrent aynı anahtar, process crash ve compensation failure için tam atomiklik varsayımı geçersizdir; olay/otomasyon yan etkileri trash ile geri alınmış sayılmaz.

İlerleme: DB'de durable operation kaydı, `(org, kind, idempotency_key)` uniqueness, request hash, state/result pointer; aynı body replay mevcut sonucu döndürür, aynı key farklı body conflict verir. Ya tek transaction+transactional event outbox ya da açık resumable operation state machine seçilmeli. “Saniyede istek sınırı”, “aynı anda işlem sınırı” ve “idempotency” ayrı özelliklerdir.

Kabul: Concurrent aynı 50-item request tek task set'i üretir; kill/retry çift görev üretmez; quota retry'ı önceki sonucu engellemez; ledger/audit başarısızlığı görünür; birikmiş compensation işlemi recover edilebilir.

### I03 — MCP endpoint'i standart MCP değil — IMPORTANT: entegrasyon vaadi

`handlers/mcp.go` özel HTTP `{tool, params}` isteğini `{tool, result}` çıktısına çeviriyor. JSON-RPC envelope, standart method/protocol negotiation ve tool discovery akışı görünmüyor. Router org altında endpoint bağlarken gereksinim daha farklı global path tarif ediyor; yorum tool sayıları da kodla aynı değil.

Öneri: Ya yüzeyi “AI tool API” olarak doğru adlandır ya da hedef client'ların desteklediği MCP sürümünü pinleyerek standart protokol/transport/auth/discovery uyumluluğunu uygula. Güncel specification yeni sürüme yönleniyor; eski lifecycle varsayımları otomatik taşınmamalı. HTTP POST read tool'u, yalnızca method yüzünden write-scoped key gerektirmemeli; authorization tool etkisine göre belirlenmeli. Rate limiter tek başına concurrency cap değildir. [Güncel MCP specification](https://modelcontextprotocol.io/specification/latest).

Kabul: Seçilen resmi client ile tool discovery/call, hata envelope, read-only key, iptal/timeouts ve version compatibility testleri geçer. İlk KOBİ sürümü için MCP, email/calendar kadar yüksek öncelikli değildir.

## 5. Rakip araştırması: kullanıcıların bugün karşılaştıracağı temel

Aşağıdaki fiyatlar araştırmada görülen sayfanın gösterimidir; vergi, kur, kampanya, team size ve ödeme dönemi eşitlenmeden doğrudan ucuz/pahalı sonucu çıkarılmaz. Rakip sayfaları satıcı iddiasıdır, bağımsız performans testi değildir. monday sayfası AUD, Notion sayfası KRW gösterdiği için bunlara USD fiyatı uydurulmadı.

| Ürün | Doğrulanan temel | Fluxboard için çıkarım |
|---|---|---|
| Asana | Starter $10.99 yıllık ödeme karşılığı kullanıcı/ay; aylık $13.49. Forms, custom fields, templates, timeline, otomasyon. Advanced $24.99 yıllık karşılığı; portfolio, workload, approvals | Sadece kanban + form yeterli değil; kurulum ve onay döngüsü üstün olmalı. [Resmi planlar](https://asana.com/pricing) |
| monday.com | Templates/custom columns; ücretli katmanlarda automation/integration kotası, timeline/calendar, guest; daha üstte time/resource/portfolio | Özel alanlar kullanıcıya rapor ve süreç olarak değer üretmeli. Sayfa AUD gösterdi; USD fiyat kıyası yapılmadı. [Resmi planlar](https://monday.com/pricing) |
| ClickUp | Unlimited $7 yıllık karşılığı / $10 aylık; Business $12 yıllık karşılığı / $19 aylık. Form, custom field, time, Gantt, entegrasyon; daha ileri automation/reporting | Özellik sayısı yarışında dezavantaj; günlük akışın basitliği ve toplam maliyet açıklığı öne çıkarılmalı. [Resmi planlar](https://clickup.com/pricing) |
| Trello | Free 10 collaborator ve 10 board; Standard $5 yıllık karşılığı / $6 aylık; Premium $10 yıllık karşılığı / $12.50 aylık. Inbox/capture, automation, paid custom fields/views | “Basit board” iddiasında güçlü rakip. Kolay başlangıç, recurring ve takip edilen talepler şart. [Resmi planlar](https://trello.com/pricing) |
| Notion | Forms, databases/properties, calendar; ücretli katmanlarda customization/connections; Business AI ve daha ince database permissions | Doküman+görev birlikteliği güçlü. Başlangıçta tam wiki kopyalamak yerine işi bitirme/takip etme farkı kurulmalı. KRW gösterimi nedeniyle USD fiyat verilmedi. [Resmi planlar](https://www.notion.com/pricing) |
| Microsoft Planner | Temel Planner Microsoft 365'e dahil; Plan 1 $10 kullanıcı/ay, yıllık ödeme; Teams ile çalışma ve structured planning | Microsoft kullanan ekiplerde ek ürün maliyeti sorgulanacak. Outlook/Teams'e bağlanmayan ürünü benimsetmek zor olabilir. [Resmi planlar](https://www.microsoft.com/en-us/microsoft-365/planner/microsoft-planner-business-plans-and-pricing) |
| Linear | Free unlimited members/250 issues/2 teams; Basic $10 ve Business $16 kullanıcı/ay yıllık ödeme karşılığı; cycles/initiatives, requests, API/webhook, AI/MCP | Yazılım odaklı benchmark. AI veya task dependency tek başına ayrışma değil. [Resmi planlar](https://linear.app/pricing) |
| Jira | Resmi feature sayfasında goals, planning, dependencies, automation/reporting ve ekipler arası çalışma | Güçlü konfigürasyon referansı; Fluxboard bunu daha az kurulumla çözmeyi deneyebilir. Fiyat sayfası okunabilir fiyat vermedi; sayısal fiyat iddiası yok. [Resmi özellikler](https://www.atlassian.com/software/jira/features) |
| Plane | Sayfada Pro $6/seat/ay, Business $13/seat/ay; views, cycles/modules/pages; ücretli time/templates/integrations; Business recurring/intake/workflow/customer | Küçük alternatiflerin de genişlediğini gösteriyor. Benzer stack/feature listesi savunulabilir fark değil. Dönem anahtarı kesin okunmadığı için fiyatın billing cadence'i iddia edilmiyor. [Resmi planlar](https://plane.so/pricing) |

### Rekabetten çıkarılan karar

Bu araştırma rakip kullanıcılarının şikâyet oranını veya Fluxboard için talep büyüklüğünü ölçmedi. “Rakipler karmaşık, kullanıcılar terk ediyor” bir kanıt olarak kullanılmamalı.

Ancak resmi özellik listeleri iki net eşiği gösteriyor: temel görev görünümleri, form/custom field/automation artık yaygın; AI da yaygınlaşmış. Bu nedenle önerilen üstünlük **aynı işin daha az kurulumla, daha az takip yüküyle ve daha açık kontrolle tamamlanması**. Bu bir konumlandırma hipotezidir; aşağıdaki pilot ölçümleriyle sınanmalıdır.

Fluxboard'un teknik özellikleri — RLS, idempotency, audit — önemli altyapı güvenceleridir. Müşteri bunları “verimlilik farkı” olarak değil “verime güvenebilmek” için bekler. Pazarlamada bunlar gerçek kanıta çevrilmeli; üstteki ürün vaadiyle karıştırılmamalı.

## 6. Mevcut özellikler nasıl ilerletilmeli?

### 6.1 Board, list, calendar ve timeline

**Mevcut:** Kanban/List/Calendar/Timeline. Timeline sabit 14 haftalık pencere; start date yoksa creation date başlangıç sayılıyor. Board route'u kolonlar üzerinden tüm task'ları topluyor; ilgili SQL listeleri limitsiz. Dependency endpoint'i bulunması timeline'ın tam dependency-aware Gantt olduğu anlamına gelmiyor.

**İyileştirme:** State category (`backlog/active/done/canceled`) tanımla; sıralamayı semantik durumdan ayır. Shared/personal saved views ve filtre presetleri; doğru empty/error durumları; klavye ile move/edit; tek günlük “benim işlerim” görünümü. Büyük board için ölçülmüş ihtiyaçla pagination ve virtualization. Timeline'da tarih zoom/navigation, milestone ve bağımlılık gösterimi daha sonra eklenebilir.

**Kabul:** Done kolonunu taşıma completion'ı değiştirmez; tarih filtresi/görünümler aynı task set'i gösterir; bağlantı/409 hatasında optimistic değişim geri alınır ve neden görünür; 1k/5k task fixture'larında payload, API ve interaction süresi kaydedilir. Bunlar benchmark hedefleri, mevcut kapasite iddiası değildir.

### 6.2 Proje ve görev şablonları

**Mevcut:** `web/lib/board/templates.ts` client'ta project oluşturup kolon/label adımlarını ayrı HTTP çağrılarıyla yapıyor. Aradaki hata kısmi kurulum bırakabilir.

**İyileştirme:** Server-side, versioned süreç template'i: kolon semantiği, task/checklist, custom fields, form, recurrence, approval ve başlangıç rolleri. İlk paketler: müşteri talebi; kampanya; operasyon checklist'i; çalışan onboarding. Template'in canlı projeyi sonradan nasıl etkilediği açık olmalı; otomatik destructive sync yapılmamalı.

**Kabul:** Aynı template retry'ında tek proje set'i; ağ kesintisinde tamamlanabilir/resumable durum; kullanıcı 10 dakikada şablon seçip 3 görev oluşturur, bir kişiyi davet eder ve ilk sonucu görür. 10 dakika bir ürün hedefidir, ölçülmüş sonuç değildir.

### 6.3 Public intake formları

**Mevcut:** Token-based form görev oluşturur; submitter adı/e-postası description içine yazılır, `created_by` form creator'dır.

**İyileştirme:** Ayrı submission/request kaydı: source, gerçek submitter, received timestamp, field schema/version, request→task bağlantısı, owner ve SLA tarihi. Form alanları custom field'larla bağlanmalı. Acknowledgment ve durum takibi, internal comment ile external response ayrımı ve spam kontrolü eklenmeli. Uygun tenant toplam kotası, tekrar teslim dedup ve token revoke denetlenmeli.

**Kabul:** Form submitter'ı audit'te çalışan kullanıcısı gibi gösterilmez; yinelenen teslim kontrollü davranır; external kullanıcı internal yorum/ekleri okuyamaz; şema güncellenince eski cevaplar bozulmaz.

Linear'ın customer request modeli feedback'i issue/project ve customer bilgisine bağlayan güçlü bir örnek. Fluxboard'un ilk sürümünde kapsamlı CRM yerine request'in sonucuna geri bildirim yeterli olabilir. [Linear customer requests](https://linear.app/docs/customer-requests).

### 6.4 Otomasyonlar

**Mevcut:** Üç olay tetikleyicisi ve dört aksiyon; evaluator best-effort hataları loglayıp sürdürür. Bazı aksiyonlar doğrudan repository update yapıyor; normal task servisindeki activity/notification/event yan etkileriyle aynı davranış otomatik varsayılmamalı.

**İyileştirme:** Önce run history, status, error/retry ve actor attribution. Sonra zaman tetikleyicileri: due yaklaşınca, overdue olunca, recurrence zamanı, onay beklerken. Condition'lar basit ve sınırlı; çok adımlı graph editor ilk adım değil. Execution, diğer task writes ile aynı domain policy ve event pipeline'ını kullanmalı; loop guard ve org action bütçesi olmalı.

**Kabul:** Başarısız kural UI'da görünür; retry yan etkileri çoğaltmaz; otomasyon başka tenant/projeye izinsiz yazamaz; sonsuz tetik zinciri durur; kullanıcı “bu task neden değişti?” sorusuna run link'iyle yanıt alır.

### 6.5 Custom fields

**Mevcut:** Project-scoped typed definitions, contributor value writes; field type immutable.

**İyileştirme:** Table/list görünümünde alanlar, filter/group/sort, form eşlemesi ve template reuse. Select option string yerine stabil option kimliği tercih edilmeli; rename/delete'nin eski değerleri nasıl etkilediği açık olmalı. Formula field ve sınırsız schema esnekliğini ertele; para alanı gerekirse integer minor unit + currency, “number” adıyla float bütçe hesabı yapma.

**Kabul:** Alanlar API/UI/export'ta aynı value tipinde; type/option değişimi eski veriyi sessiz bozmaz; unauthorized field write reddedilir; customer-facing görünüm internal field'ı sızdırmaz.

### 6.6 Zaman takibi ve kapasite

**Mevcut:** Task timer/manual time entries. Bu, timesheet, faturalandırılabilir süre ve ekip kapasite yönetiminin tamamı değildir. AI kapasite sinyali kişi başına task sayısına dayanıyor.

**İyileştirme:** Haftalık timesheet, düzeltme/approval, estimate-vs-actual ve proje toplamı. Kapasite için çalışma günü/saatleri, izinler ve estimate gereklidir; bir kişinin 12 kısa task'ı ile 3 uzun task'ı eşit kapasite değildir. İnsan performans sıralaması yerine ekip risk/planlama amaçlı tasarla.

**Kabul:** Timer restart/device değişiminde tutarlı; concurrent start çift aktif timer bırakmaz; timezone/overnight/overlap senaryoları tanımlı; estimate ve availability yoksa sahte utilization yüzdesi üretilmez.

### 6.7 Sprint ve dependency

**Mevcut:** Sprint plan/start/complete, task atama, final column'a göre completed count. `AssignSprint` ve completion task updates loop içinde ayrı repository çağrıları yapıyor.

**İyileştirme:** Toplu atama/completion'ın tek operasyon olarak tutarlılığı; start-scope snapshot; scope-change history; semantik done state; cycle-safe dependency. Points/velocity ve burndown ancak yazılım müşterilerinde talep oluşursa. Genel KOBİ navigasyonunda sprint zorunlu kavram olmamalı.

**Kabul:** Toplu atamanın ortasında yanlış task ID tüm operasyonu bozmadan kısmi başarıya dönüşmez; açık politika atomiklik veya görünür partial-result; kolon reorder metrikleri değiştirmez; completed sprint'in geçmişi yeniden yorumlanmaz.

### 6.8 Bildirim ve gerçek zaman

**Mevcut:** SSE stream, in-app notification, preferences ve job altyapısı.

**İyileştirme:** Task deep-link/by-number resolver ile board'un tamamını yüklemeden hedefe gitme; duplicate/coalescing; quiet hours; daily/weekly digest; unread count tutarlılığı. Bağlantı kopunca stale cache'in görünür ve toparlanabilir olması; permission değişiminde stream/data yetkisi yeniden değerlendirilmesi.

**Kabul:** Assignment/mention/task change bildirimleri doğru kişiye; removed member yeni event almaz; 5 dakika replay penceresi aşılınca full resync; aynı event tekrarında unread badge çift artmaz. SMTP teslim/bounce testi local Mailpit'in yerini alır.

### 6.9 AI'nin mevcut altyapısı

**Mevcut:** Mock provider; parse sonucu prose satırlarından ayrılıyor. Chat model çağrısına message/history yollar; live task retrieval görünmüyor. Risk sinyalleri overdue, high-priority unassigned ve assignee task count; gereksinimdeki dependency churn/scope frequency hesapları bu fonksiyonda yok. Özet, done state ayırmadan due date geçmiş görevleri overdue sayabiliyor.

**İyileştirme sırası:** B06/I02 gider → semantik state ve doğru metrics → gerçek provider → bounded structured output → provenance → kullanıcı review/apply. Owner-level policy ve token/cost reservation; org ve user/project budget; timeout/provider outage/cancel; Türkçe/İngilizce tarih anlamı. Feature flag'ler yalnızca admin panelinde değil müşterinin anlaşılır kontrol yüzeyinde olmalı.

**Kabul:** Mock üretimde gerçek AI gibi sunulmaz; yetkisiz proje context'e girmez; structured output şema doğrulaması ve assignee/date referans kontrolü geçer; kullanıcı task writes'ı önceden görür; rapordaki her sayının hesap ve kaynak task link'i vardır. Kullanıcı içerikleri prompt injection açısından untrusted veri olarak işlenir. [OWASP GenAI güvenliği](https://owasp.org/www-project-top-10-for-large-language-model-applications/).

### 6.10 API, admin ve ürün operasyonu

**Mevcut:** Elle yazılan frontend client; public OpenAPI JSON'unda üç path; org keys/admin/audit yüzeyleri.

**İyileştirme:** API sözleşmesini route/DTO/auth/error/entitlement bazında tamamla; spec drift gate ve gerekiyorsa bundan üretilen client. API key expiration/rotation/scopes; servis kimliği; support için audited read-only access; müşterinin kendi data export ve billing troubleshooting ekranı.

**Kabul:** Her exposed route'un sözleşmesi ve negative auth testi; admin işlemleri doğru actor/tenant ile loglanır; export row count/source IDs uyuşur; platform-role değişimi erişimi kaldırır. Sadece OpenAPI dosyasının varlığı entegrasyon kalitesi değildir.

## 7. Eklenmesi önerilen yeni özellikler

P0 teknik çıkış kapıları tamamlanmadan bu paketlerin tamamına başlanmamalı. Aşağıdaki sıralama pazardan ölçülmüş RICE puanı değildir; belirtilen hedef müşteri ve mevcut temel üzerinden ürün yargısıdır.

| Öncelik / paket | Müşteriye değer | Minimum kapsam ve kabul | Bağımlılık |
|---|---|---|---|
| P1 — İçe/dışa aktarma | Mevcut Excel/CSV/Trello işini taşır; çıkışta kilitlenmez | CSV mapping preview, date/person/status eşlemesi, hata raporu, batch retry; task/comment/attachment metadata JSON export ve kontrollü attachment download manifest'i; aynı import retry çift kayıt yapmaz | Domain create policy, durable operation, object permissions |
| P1 — Tekrarlayan işler | Haftalık rapor, kontrol, onboarding adımı unutulmaz | Gün/hafta/ay, timezone, tamamlanınca veya takvim bazlı açık recurrence; occurrence identity; pause/skip/edit scope; ay sonu ve DST'de duplicate yok | Jobs/outbox recovery, state category |
| P1 — Onay ve teslim | İşin kim tarafından ne zaman kabul edildiği bellidir | Requested/approved/changes requested, approver, deadline, artifact/version, audit; aynı kişi kendi işini onaylayabilir mi policy; task tekrar açılınca onay semantiği açık | Roles, task versions, notifications |
| P1 — Süreç başlangıcı | Kurulumu azaltır | Kullanım amacına göre 3–4 template, guided create, invite, first task; admin tüm modülleri öğrenmek zorunda değil | Server templates, plan catalog |
| P1 — Talep inbox'ı | Dış talebi takip edilebilir işe dönüştürür | Form request lifecycle, owner/priority/ack; linked task ve completion update; internal/external ayrımı | Forms schema, delivery |
| P1 — Günlük çalışma ekranı | “Bugün ne yapmalıyım?” sorusunu çözer | My day, approaching due, waiting on others, saved views; görev kaynakları açık, sırayı kullanıcı değiştirebilir | Correct status/date/filter semantics |
| P2 — Takvim ve e-posta bağlantıları | Mevcut araçları terk etmeden kullanım | Önce e-posta capture ve seçilen bir takvim; event↔task identity, disconnect/revoke, retry/conflict politikası; sonra ikinci ecosystem | OAuth scopes/token encryption, integration ledger |
| P2 — Ekip workload / proje portföyü | İş yükü ve teslim riskini gösterir | Multi-project open work, estimate/time horizon/availability, overdue/blocked, last updated; eksik veride N/A | Correct analytics, estimate/availability |
| P2 — Müşteri/guest çalışma alanı | Dış paydaş güvenli yorum/onay verir | Invite edilmiş proje/task scope, explicit shared fields/attachments; hiçbir internal yorum otomatik paylaşılmaz; seat policy açık | Private project gates, approvals |
| P2 — Kaynaklı durum raporu | Toplantı ve takip yükünü azaltır | Önce deterministic haftalık rapor; değişiklikler/blockers/next steps ve task link'leri; sonra AI anlatım; owner review | Analytics/recovery/permission correctness |
| P3 — Knowledge/docs ve ileri workflow | Bilgiyi işe bağlar | Hafif project brief/decision log; gerekiyorsa workflow extension. Tam wiki/chat/whiteboard suite'i başlangıçta yapma | Pilot demand |
| P3 — Enterprise ihtiyaçları | Büyük müşteri satışını mümkün kılar | SAML/SCIM, domain policies, advanced audit/export, contractual SLA; müşteri talebiyle | Operasyon kapasitesi ve scope ADR |

### Takvim ve iletişim entegrasyonu tasarımı

İlk entegrasyon Google mı Microsoft mu olmalı, pilotların kullandığı araçlara göre seçilmeli. Genel KOBİ hedefinde yalnızca GitHub'a yatırım yapmak doğru ilk varsayım değil. Mevcut Google login bir Google Calendar entegrasyonu değildir.

Başlangıçta task due dates için revocable read-only calendar subscription veya tek yönlü event yayınlama yeterli olabilir. İki yönlü sync ayrı karmaşıklık: external event ID, source ownership, version, deleted tombstone, timezone, permission revoke, retry ve conflict policy gerekir. Google incremental sync token ve Microsoft delta mekanizmaları bu tasarımın resmi girdileridir; her dakika tüm takvimi çekmek doğru varsayılan değildir. [Google Calendar sync](https://developers.google.com/workspace/calendar/api/guides/sync), [Microsoft Graph delta](https://learn.microsoft.com/en-us/graph/delta-query-overview).

E-posta capture için sender ve reply chain doğruluğu, attachment güvenliği, duplicate Message-ID/delivery ID ve project routing gerekir. Sonraki Slack/Teams entegrasyonu, pilot kullanım verisine göre seçilmeli. Outgoing webhook eklenirse signed payload, delivery history, backoff/dead letter ve safe outbound destination kontrolü gerekir; arbitrary URL çağrıları güvenlik yüzeyi açar.

## 8. Rakiplerden daha iyi sunulabilecek deneyim: ölçülebilir üç vaat

### Vaat 1 — İlk gün çalışan süreç

Bir operasyon yöneticisi şablon seçer, mevcut CSV'sini preview ederek aktarır, üç kişiyi davet eder, tekrarlayan işi ve onaycısını belirler. Ürün konfigürasyon dili öğrenmek yerine gerçek işi başlatır.

**Hedef hipotez:** Davet edilen ekiplerin çoğunluğu ilk 24 saatte en az bir görevi birden fazla kişiyle ilerletir; median first-value süresi 10 dakikanın altında. Kayıt sayısından ziyade collaborative first value ölçülmeli.

### Vaat 2 — İşin nerede beklediği açık

Yönetici “kaç task var?” yerine “hangi talep onayda, kimden yanıt bekleniyor, hangi teslim riske girdi?” cevabını görür. Her uyarının nedeni, son değişikliği ve doğrudan kaynak task'ı bulunur. Completed task eski due date yüzünden overdue görünmez.

**Hedef hipotez:** Haftalık manuel durum hazırlama süresinde ≥30% azalma; hatalı risk uyarıları pilotlarda işaretlenir ve azaltılır. Bu oran bir satış iddiasına ancak aynı ekiplerin önce/sonra ölçümüyle dönüşür.

### Vaat 3 — Kullanıcı fiyatı ve kontrolü anlar

Misafir davetinin fiyat etkisi, plan limiti ve AI tüketimi açık. Gereksiz otomatik AI işlem veya normal task activity için sürpriz API ücreti yok. Kullanıcı verisini alabilir, AI'yi kapatabilir, automation'ın neden yaptığı değişikliği görebilir.

**Hedef hipotez:** Pilotların plan toplamını yardım almadan doğru hesaplaması; limit aşımı öncesi uyarı; AI action preview ve kalan bütçe; billing itirazlarında source records'a ulaşılabilmesi.

Bu vaatler daha çok özellikten daha savunulabilir olabilir; fakat müşteri görüşmesi ve kullanım kanıtı henüz yoktur. Türkçe/İngilizce UI, uygun tarih/timezone ve yerel onboarding bir avantaj hipotezi olabilir. Sırf Türkçe dil desteği kalıcı rekabet engeli sayılmaz.

## 9. Fiyatlandırma ve maliyet modeli

**Öneri:** İlk ticari sürümde öngörülebilir ekip paketi veya açık koltuk fiyatı; normal web kullanımı için metered API ücretini kullanıcıya taşımamak. AI ve çok yüksek storage gibi değişken maliyetler açık dahil bütçe ve üst sınırla sunulmalı. Mevcut çok boyutlu seat/storage/API usage modeli KOBİ müşterinin satın alma kararını gereksiz zorlaştırabilir.

Tek alternatif: yüksek hacimli API/integration müşterilerinde separate metered add-on. Bunun normal kullanıcı etkileşimleriyle karışmaması gerekir. UI'ın API üzerinden çalışması, her UI çağrısının billable developer API çağrısı olması gerektiği anlamına gelmez.

Yeni dolar fiyatı bu raporda ilan edilmiyor. Rakiplerin kullanıcı başına fiyatı ile Fluxboard seed'inin org başına görünmesi muhtemel fiyatı eşitlenmeden karşılaştırma yanıltır. Ürün paketi ve gerçek maliyet belirlenmeden `$12/$49` üzerinden satışa çıkılmamalı.

**Ölçülecek aylık maliyet:**

`tenant_cost = allocated_base_infra + storage + egress + email + AI_provider + payment_cost + support_allocation`

`gross_margin = (net_revenue - direct_service_cost) / net_revenue`

Varsayımsal bir tenant için $30 net gelir ve $9 doğrudan hizmet maliyeti, %70 gross margin verir. Bu sadece hesap örneğidir; Fluxboard'un ölçülmüş maliyeti değildir. Destek/onboarding emeği ihmal edilirse küçük paket kârlı görünürken zarar ettirebilir.

Seat tanımı registered/active/billable member üzerinden açıkça seçilmeli. Daily active HLL sayısı unique activity tahminidir; sözleşmesel billable seat count ile aynı değildir. Storage için allocated vs retained vs deleted/backed-up byte ayrımı ve egress maliyeti bilinmeli. GB/GiB gösterimleri de tutarlı olmalı.

**Karar kanıtı:** 10–15 hedef ekip görüşmesi, en az 5 gerçek pilot, birkaç fiyat/package teklifine gerçek ödeme isteği. Bu sayılar önerilen araştırma planıdır, yapılmış görüşmeler değildir.

## 10. Uygulama yol haritası ve release kapıları

Takvim tarihleri tahmin değil; aşağıdaki iş sırası bağımlılık planıdır. Takım büyüklüğü, provider/şirket uygunluğu ve runtime sonuçları bilinmeden hafta/gün sözü vermek doğru olmaz. Her paket küçük PR'lara bölünmeli, mevcut repo branch/ADR/gate kuralları korunmalı.

| Sıra | Çalışma paketi | Çıkış kanıtı | Kapanmadan başlanmaması gereken |
|---|---|---|---|
| 0 | Doküman/katalog/scope kararları | Plan modeli, billable seat, state semantics, pilot sektörleri ve AI policy ADR'de | Gerçek billing implementation ve pricing ilanı |
| 1 | Prod config + supported frontend + gerçek CI | B03/B05/B08; auth/role/storage testleri staging'de | Public veri kabulü |
| 2 | Delivery/recovery/release | B04/B07/B09; restore ve fault-injection; alarm delivery; gerçek operator iletişimi | Public müşteri verisi |
| 3 | Gerçek ödeme + katalog | B01/B02; provider sandbox+kontrollü live kanıtı | Ücretli abonelik |
| 4 | Analytics/state + API sözleşmesi | I01; done/reopen metrics; contract drift gates | Risk/dashboard satış vaadi |
| 5 | Onboarding/import/templates/my-day | Birincil iş akışı nontechnical pilotla tamamlanır | Büyük pilot havuzu |
| 6 | Recurring/approval/intake | At-least-once jobs altında doğru occurrence/approval/request lifecycle | Operasyon ürünü olarak geniş satış |
| 7 | Calendar/email + workload/guest | Seçilen ilk ecosystem'in end-to-end sync ve negative permission kanıtı | Yeni integration vaatleri |
| 8 | Gerçek AI | B06/I02; eval set/cost gate/provenance; standard MCP ancak talep varsa | AI ücretli add-on ve autonomous writes |

### Release A — Kapalı beta

Kayıtlar allowlist; beklentiler açık; B03–B09'un beta için geçerli olanları kapanmış; backup/restore ve support çalışıyor. Gerçek ödeme hazır değilse paid checkout kapalı. AI hazır değilse bütün AI/MCP entrypoint'leri disabled; mock demo açıkça ayrı demo olarak etiketlenir. Sadece frontend'de gizleme yeterli değildir.

### Release B — Ücretli pilot

Gerçek ödeme ve pricing doğrulaması; küçük sayıda gerçek ekip; billing/permission/data recovery kanıtı; veri taşıma/çıkış olanağı; operasyonel destek. Rapor/AI eksik yüzeyleri açıkça devre dışı. Genel reklam öncesi retention ve destek yükü ölçülür.

### Release C — Genel kullanıma açık production

Yeni müşteri self-service onboarding; recurring/approval/request gibi seçilen ürün vaadi eksiksiz; SLO/error budget izleniyor; deployment rollback ve operator alarm tatbikatı; public status; maliyet/abuse limitleri; privacy/DPA gerçeği yansıtıyor. Paid pilot başarısı bu kapıyı kendiliğinden kapatmaz.

## 11. Ölçüm ve kabul planı

### Ürün ölçümleri

Önerilen ölçüm birimi “görev sayısı” değil **aktif ve iş birliği yapan organizasyon**. Bir aktif hafta tanımı önerisi: en az iki farklı üyenin iş değiştirdiği ve en az bir işin tamamlandığı hafta. Sektöre göre revize edilmeli; passive approver değeri yok sayılmamalı.

| Ölçüm | Amaç | Başlangıç değerlendirmesi |
|---|---|---|
| Collaborative activation | İlk değere ulaşma | create/import + invite accepted + task başka kullanıcıyla ilerledi |
| Time to first value | Kurulum yükü | Signup ile ilk ortak çalışma arasında geçen süre; auth e-posta gecikmesi ayrı |
| W1/W4 org retention | Tekrar kullanım | İlk etkinleşme kohortundan 1./4. haftada etkin org; yeterli cohort olmadan oran yorumlanmaz |
| Recurrence/approval adoption | Önerilen farkın kullanımı | İlgili kullanım ihtiyacı olan org'larda kullanım, yalnızca tüm signup'larda değil |
| Manual reporting time | Net verim | Aynı ekipte önce/sonra, görev hacmi ve dönem farkıyla birlikte |
| Billing/support load | Ticari sürdürülebilirlik | 100 aktif org başına bilet, çözüm süresi, disputed invoice ve onboarding emeği |
| AI acceptance | AI faydası | Kabul/edited/rejected öneri, time saved, yanlış bilgi ve cost/action |

Telemetry izin ve privacy metniyle uyuşmalı; task description/comment gibi müşteri içeriklerini analytics event payload'ına taşımamak gerekir.

### Operasyon hedefleri — mevcut ölçüm değildir

- Pilot SLO adayı: task read/write başarı oranı aylık %99.9; availability'nin hangi request'leri ve exclusions'ı kapsadığı yazılmalı. 30 günlük ayda %0.1 bütçe yaklaşık 43.2 dakikadır; bu doğrudan müşteriye SLA/ceza vaadi değildir.
- Mevcut dokümandaki CRUD p95 <300 ms seeded load hedefi korunup workload ve ortamla raporlanmalı; sadece local sonuç production sonucu sayılmamalı.
- E-posta/notification lag, outbox age, dead-letter, webhook ingest-to-entitlement lag, DB pool ve storage error ayrı izlenmeli.
- Alarm “dashboard var” ile kapanmaz; gerçek receiver'a teslim ve müdahale prosedürü test edilmeli.
- Tenant başına limit ve backpressure, bir yoğun müşterinin diğerlerini etkilemesini önlemeli; queue fairness ve org quotas yük testine dahil edilmeli.

SLO'lar kullanıcı etkisini ve release kararını yönlendirmeli; her ERROR logu page üretmemeli. [Google SRE SLO uygulaması](https://sre.google/workbook/implementing-slos/).

### Frontend kalite kabulü

Bu çalışmada screenshot/interaction testi yapılmadığı için mevcut tasarıma erişilebilirlik onayı verilmiyor. Önerilen hedef WCAG 2.2 AA: klavyeyle board/task/form/billing, focus ve error association, drag dışında move alternatifi, contrast, reduced motion ve küçük ekran. [WCAG 2.2](https://www.w3.org/TR/WCAG22/).

Field performance hedefleri p75 LCP ≤2.5 s, INP ≤200 ms, CLS ≤0.1; bunlar web ölçüm rehberinin iyi deneyim sınırlarıdır. Marketing ve authenticated app ayrı ölçülmeli; Lighthouse tek koşum kullanıcı gerçekliği değildir. Canvas/animation katkısı first action'ı geciktirmemeli. [Web Vitals](https://web.dev/articles/vitals).

### Golden journeys — uygulanacak doğrulama, bu araştırmada çalıştırılmadı

1. Register → gerçek verification email → login → org → invite → yeni üye kabulü → task atama/tamamlama.
2. İki tenant + aynı tenant private projede role değişimi → task/search/attachment/AI/SSE negatif erişim matrisi.
3. Checkout → signed webhook → subscription/quantity/entitlement → upgrade/downgrade → payment failed → portal → invoice.
4. Form/email request → task → approval → completion → external status; internal yorum/gizli field paylaşılmaz.
5. Recurring task → worker kill/retry → tek occurrence; outbox fail/recovery → kayıpsız iş.
6. Backup → temiz recovery ortamı → DB+migrations+RLS+object metadata/content → login/task/download; ölçülmüş RPO/RTO.
7. Concurrent task edit/move ve bulk/AI apply → conflict veya replay davranışı → bozulmayan task/event/audit durumu.
8. Calendar sync → update/delete → credential revoke → reconnect → duplicate olmayan doğru state.

Güvenlik acceptance inventory'si için OWASP ASVS yararlı bir resmi kontrol temelidir; bu rapor ASVS uygunluğu veya güvenlik sertifikası vermez. [OWASP ASVS](https://owasp.org/www-project-application-security-verification-standard/).

## 12. Şimdilik ertelenmesi gerekenler

- Native iOS/Android: önce mobil web günlük akış ve ölçülmüş ihtiyaç.
- Microservices/Kubernetes/multi-region: ilk ekiplerin işini iyileştirmez; recovery ve operatör yükünü artırabilir.
- Full chat/wiki/whiteboard/marketplace: yeterli ürün odağı ve extension ihtiyacı oluşunca.
- Tam ERP/CRM/muhasebe: mevcut task/süreç çekirdeğinin dışına çıkar; provider invoice ile müşteri muhasebe işlemleri aynı ürün değildir.
- Autonomous AI writes ve her yere chatbot: güvenilir state/context/cost/onay altyapısı önce gelir.
- SAML/SCIM ve özel mevzuat uyumu: kurumsal talep ve destek kapasitesiyle ayrı ticari paket; Google OAuth login enterprise federation değildir.
- İleri Gantt/critical path/velocity: genel KOBİ hedefinde recurring/approvals/import/calendar'dan sonra.

## 13. İlk karar toplantısının çıktıları

Uygulama öncesi şu altı karar tek dokümanda kapanmalı:

1. Pilotların ortak işi ve ilk üç süreç şablonu; ilk pilot cohort'u farklı sektörleri kapsarken aynı problem etrafında toplanmalı.
2. Organizasyon paketi mi koltuk fiyatı mı; billable seat/guest, dönem, AI/storage bütçesi ve normal API kullanım politikası.
3. Ödeme hesabı/işletme ülkesi/provider uygunluğu ve gerçek integration kapsamı.
4. Workflow state semantics; overdue/completed/cycle-time/approval/recurrence tanımları.
5. Hosting bölgesi, backup/RPO/RTO, operator, support ve privacy/subprocessor envanteri.
6. İlk calendar/communication ecosystem ve AI'nin ilk sürümde açık mı kapalı mı olduğu.

Bu kararlar ürün lideri/işletme sahibi tarafından kabul edilince teknik paketler ADR/FR kimlikleriyle backlog'a çevrilmeli. Rapor yeni requirement ID'lerini kabul edilmiş gibi atamıyor; mevcut `FR-BILL`, `FR-TEN`, `FR-TASK`, `FR-AN`, `FR-AI`, `FR-API` kapsamları ve ADR-025 başta olmak üzere ilgili kararlar referans alınmalı.

## 14. Kanıt ve kaynak dizini

### Yerel dosyalar

| Konu | İncelenen kaynak |
|---|---|
| Ürün kapsamı / eski hedef | [overview](../00-PROJECT-OVERVIEW.md), [gereksinimler](../01-FUNCTIONAL-REQUIREMENTS.md), [build tracker](../build/README.md), [README](../../README.md) |
| Route ve yetki zinciri | [router](../../backend/internal/interface/http/router.go), [tenant middleware](../../backend/internal/interface/http/middleware/tenant.go), [proje erişimi](../../backend/internal/usecase/projectuc/service.go) |
| Ödeme gerçek/stub ayrımı | [Stripe gateway](../../backend/internal/infrastructure/stripe/client.go), [seed](../../backend/cmd/stripeseed/main.go), [billing servis](../../backend/internal/usecase/billinguc/service.go) |
| Plan ayrışması | [marketing](../../web/lib/marketing/plans.ts), [billing referansı](../../web/lib/billing/plans.ts), overview ve seed |
| Gerçek test/skip | [CI](../../.github/workflows/ci.yml), [integration test](../../backend/internal/infrastructure/postgres/webhook_repo_integration_test.go), [testing spec](../12-TESTING.md) |
| Outbox | [claim SQL](../../backend/internal/infrastructure/postgres/queries/outbox.sql), [billing jobs](../../backend/internal/interface/jobs/billing.go) |
| Startup fallback / mail | [API wiring](../../backend/cmd/api/main.go), [config](../../backend/internal/config/config.go), [mailer](../../backend/internal/infrastructure/mailer/mailer.go) |
| AI/read/apply/MCP | [AI servis](../../backend/internal/usecase/aiuc/service.go), [apply](../../backend/internal/usecase/aiuc/apply.go), [MCP handler](../../backend/internal/interface/http/handlers/mcp.go), [ADR-025](../adr/ADR-025-ai-layer.md) |
| RLS/private visibility ayrımı | [project repo](../../backend/internal/infrastructure/postgres/project_repo.go), [project SQL](../../backend/internal/infrastructure/postgres/queries/projects.sql), [RLS](../../backend/migrations/0008_projects_rls.up.sql) |
| Metrics doğruluğu | [stats rollup](../../backend/internal/infrastructure/postgres/stats_repo.go), [analytics servis](../../backend/internal/usecase/analyticsuc/service.go) |
| Deploy/recovery/alarm | [Compose](../../deploy/docker-compose.yml), [Prometheus](../../deploy/prometheus/prometheus.yml), [restore](../runbooks/restore.md), [infra spec](../10-INFRA-DEVOPS.md) |
| Frontend destek / timeline | [package](../../web/package.json), [timeline](../../web/components/board/timeline.tsx), [task SQL](../../backend/internal/infrastructure/postgres/queries/tasks.sql) |
| Hukuki metinler | [privacy](../../web/app/(public)/legal/privacy/page.tsx), [DPA](../../web/app/(public)/legal/dpa/page.tsx), [terms](../../web/app/(public)/legal/terms/page.tsx) |

Dış kaynaklar ilgili karar/bulgunun yanında doğrudan bağlandı. Güncel araştırma tarihinde resmi ürün dokümanları kullanıldı; fiyat/plan/sürüm/provider politikaları uygulama ve satış tarihinde yeniden kontrol edilmelidir.

**Teslim kapsamı:** Yalnızca araştırma raporu. Uygulama, migration, deploy ve kullanıcıya açık metinler değiştirilmedi; test/CI/production kabulü verilmedi.
