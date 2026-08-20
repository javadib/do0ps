<div dir="rtl">

# do0ps

> یک سرور MCP که به چت‌بات اجازه می‌دهد زیرساخت شما را اداره کند.

**[English →](README.md)**

---

## فهرست مطالب

1. [معرفی](#sec-1)
2. [مفاهیم](#sec-2)
3. [ابزارها و تکنولوژی‌های استفاده‌شده](#sec-3)
4. [معماری](#sec-4)
5. [ساختار مخزن](#sec-5)
6. [نصب و راه‌اندازی](#sec-6)
7. [مرجع پیکربندی](#sec-7)
8. [رابط MCP](#sec-8)
9. [اتصال کلاینت MCP](#sec-9)
10. [Skill‌ها](#sec-10)
11. [MCPهای متصل](#sec-11)
12. [APIهای ارائه‌دهنده (پارس‌پک)](#sec-12)
13. [روند توسعه](#sec-13)
14. [وضعیت فعلی پروژه و کاستی‌های شناخته‌شده](#sec-14)
15. [نقشه راه](#sec-15)
16. [مشارکت](#sec-16)

> **این مستند مربوط به کدام برنچ است؟** توسعه روی برنچ **`develop`** انجام می‌شود، نه روی برنچ پیش‌فرض
> `master`. این مستند `develop` را توصیف می‌کند که جلوتر از `master` است. بخش
> [روند توسعه](#sec-13) را ببینید.

---

## <a id="sec-1"></a>۱. معرفی

**do0ps** ارائه‌دهندگان هاستینگ و PaaS — با شروع از ارائه‌دهندهٔ ایرانی **پارس‌پک** — را از طریق پروتکل
**MCP (Model Context Protocol)** به چت‌بات‌ها وصل می‌کند. هر قابلیت ارائه‌دهنده به شکل یک ابزار (Tool) در MCP
منتشر می‌شود تا دستیارهایی مثل Claude یا ChatGPT/Codex بتوانند از دل زبان طبیعی، کار واقعیِ زیرساخت را از طرف
کاربر انجام دهند: «یک سرور اوبونتو ۲ گیگابایتی در تهران بساز و api.example.com را به آن اشاره بده.»

شرط‌بندیِ اصلیِ طراحی این است که **به هیچ کد NLU اختصاصی نیازی نیست**. مدلِ فراخوان خودش «۲ گیگ رم» را به
`ram_mb: 2048` تبدیل می‌کند — به شرطی که توضیحات ابزار و پارامترها به‌قدر کافی دقیق باشد. بنابراین تمرکز تلاش
روی کیفیت اسکیما است، نه روی مهندسی پرامپت.

دو لایه روی این سرور قرار می‌گیرد و هر دو حالا وجود دارند:

| لایه | چیست | کجاست |
| --- | --- | --- |
| **ابزارهای MCP** | ۱۴۷ ابزار در حوزهٔ VM، شبکه، CDN و SSL، هرکدام با JSON Schema کاملاً توصیف‌شده | `../internal/adapters/mcp` |
| **Skill / سیستم‌پرامپت** | راهنمای هماهنگی میان ابزارها و قواعد کسب‌وکار («قبل از ساخت ساب‌دامین، زون DNS را پیدا کن») | [`../skills/parspack-infra/SKILL.md`](../skills/parspack-infra/SKILL.md) |

پروژه قرار است متن‌باز باشد و هم برای تیم‌های DevOps و هم برای کاربران نهاییِ غیرفنی که فقط از طریق چت‌بات با آن
سروکار دارند قابل استفاده باشد.

**وضعیت فعلی:** آداپترهای پارس‌پک، Use Caseها و ابزارها پیاده‌سازی و در همهٔ پکیج‌ها یونیت‌تست شده‌اند،
`go test ./...` سبز است، و سرور هم از باینری ساده، هم از داکر و هم به‌صورت MCP bundle قابل نصب بالا می‌آید.
برای دیدن آنچه هنوز کم است بخش [وضعیت فعلی پروژه](#sec-14) را ببینید.

---

## <a id="sec-2"></a>۲. مفاهیم

### MCP (Model Context Protocol)

پروتکلی باز که به چت‌بات اجازه می‌دهد ابزارهای بیرونی را کشف و فراخوانی کند. کلاینت ابتدا فهرست ابزارها را
می‌گیرد (`tools/list`) و سپس یکی را با آرگومان‌های ساخت‌یافته صدا می‌زند (`tools/call`). do0ps یک **سرور** MCP
است: ابزارهای زیرساختی را منتشر و اجرا می‌کند.

### ابزارها و کیفیت اسکیما

هر عملیات ارائه‌دهنده به یک ابزار تبدیل می‌شود که در JSON Schema آن، هر ویژگی توضیح خوانا، واحد و نمونه دارد؛
مثلاً `ram_mb: "RAM in megabytes, e.g. 2048 for 2GB"`. همین است که به مدل اجازه می‌دهد پارامترهای درست را از یک
جمله بیرون بکشد.

### دو لایهٔ احراز هویتِ کاملاً بی‌ارتباط

این دو مدام با هم اشتباه گرفته می‌شوند، بنابراین پروژه آن‌ها را سفت‌وسخت از هم جدا نگه می‌دارد:

| | احراز هویت دسترسی به MCP | اعتبارنامهٔ ارائه‌دهنده |
| --- | --- | --- |
| **از چه محافظت می‌کند** | خودِ سرور do0ps | حساب شما در پارس‌پک |
| **کجا حمل می‌شود** | هدر `Authorization: Bearer <token>` | آرگومان‌های فراخوانی ابزار (`api_key`, `secret_key`) |
| **چه کسی تنظیمش می‌کند** | ادمین سرویس، از طریق `MCP_AUTH_TOKENS` | کاربر نهایی؛ در نشست چت‌بات او نگه‌داری می‌شود |
| **ذخیره در سمت سرور** | بله — به صورت خلاصهٔ SHA-256 و در حافظه | **هرگز.** نه انباره‌ای برای اعتبارنامه، نه رمزنگاری در حالت سکون؛ هرگز در دیتابیس یا لاگ نوشته نمی‌شود |

اعتبارنامهٔ ارائه‌دهنده در هر فراخوانی می‌آید، مصرف می‌شود و دور ریخته می‌شود. تایپ `ProviderCredentials` حتی
`String()`/`GoString()` را بازنویسی کرده تا `REDACTED` چاپ شود؛ پس یک `%v` سهوی هم نمی‌تواند آن را لو بدهد.

### عملیات سریع در برابر عملیات طولانی

هر Use Case باید اعلام کند از کدام دسته است:

- **سریع** (چند ثانیه — تغییرات DNS، گرفتن فهرست‌ها، بیشتر تنظیمات CDN): Use Case کار را به یک worker می‌سپارد
  و در همان فراخوانی ابزار منتظر نتیجه می‌ماند. فراخوان‌کننده اصلاً نمی‌فهمد صفی وجود دارد.
- **طولانی** (چند دقیقه — ساخت سرور، ساخت Load Balancer، اسنپ‌شات، بازیابی VM): ابزار بلافاصله با یک
  `operation_id` و وضعیت `pending` برمی‌گردد. یک worker در پس‌زمینه فراخوانی را انجام می‌دهد و تا آماده‌شدن
  منبع، وضعیت را poll می‌کند. فراخوان‌کننده پیشرفت را با `get_operation_status` دنبال می‌کند.

چهار نوع job طولانی هستند: `provision_server`، `provision_load_balancer`، `create_snapshot` و `restore_vm`.
عملیات طولانی هرگز فراخوانی ابزار را بلاک نمی‌کند، چون پاسخ چنددقیقه‌ای در MCP یعنی تایم‌اوت سمت کلاینت.

### Job، Operation و آشتی‌دهی (Reconciliation)

**Job** رکورد داخلی است (وضعیت، تعداد تلاش، زمان‌بندی retry، payload) و **Operation** تصویر کوچک‌ترِ رو به
کاربر از همان (`pending` / `running` / `succeeded` / `failed` به‌همراه نتیجه). Jobها در SQLite ذخیره می‌شوند تا
یک ری‌استارت آن‌ها را از بین نبرد.

**Idempotency اینجا یک مسئلهٔ درستی است، نه بهینه‌سازی.** اگر هنگام بالا آمدن سرویس، jobای در وضعیت `running`
پیدا شود، ممکن است یعنی فراخوانی ارائه‌دهنده لحظه‌ای پیش از کرش موفق شده باشد. تکرار کورکورانهٔ آن یعنی ساختن یک
سرور تکراری که کسی باید پولش را بدهد. پس do0ps هرگز کورکورانه retry نمی‌کند:

۱. هنگام استارت، `Recovery` هر job ناتمام را «interrupted» علامت می‌زند و آن را در وضعیت غیرنهایی نگه می‌دارد.
۲. اعتبارنامهٔ لازم برای بررسی از بین رفته است (طبق طراحی، هرگز ذخیره نشده بود).
۳. اولین فراخوانی `get_operation_status` که اعتبارنامه همراه داشته باشد job را آشتی می‌دهد: از ارائه‌دهنده
می‌پرسد آیا آن منبع وجود دارد یا نه، و بعد عملیات را «موفق» یا «قابلِ تکرارِ امن» نهایی می‌کند.

retry داخل یک پروسهٔ زنده هم همین‌طور کار می‌کند: `createOrAdopt` پیش از صدور دستور ساختِ دوم، دنبال منبعِ
موجود با همان نام می‌گردد.

### Retry و Backoff در دو سطح

- **داخل یک فراخوانی ارائه‌دهنده** — کلاینت پارس‌پک درخواستی را که با وضعیت قابل‌تکرار شکست خورده، با backoff
  نمایی دوباره می‌فرستد و بعد تسلیم می‌شود.
- **دور تا دور یک job پس‌زمینه‌ای** — worker pool هندلر شکست‌خورده را تا `maxAttempts` (پیش‌فرض **۵**) تکرار
  می‌کند؛ `Reschedule(...)` را با زمان تلاش بعدی ذخیره می‌کند و job را با یک تایمر دوباره در صف می‌گذارد. چون
  `NextRetryAt` پیش از انتظار ذخیره شده، کرش وسط backoff باعث گم‌شدن retry نمی‌شود.
- **یک sweep بازیابی** هر ۳۰ ثانیه (`WithSweepInterval`) با `JobRepository.ListDue` انبارهٔ job را دوباره
  می‌خواند و کارهای pending ای که pool رهایشان کرده دوباره در صف می‌گذارد — حالتی که وقتی پیش می‌آید که
  دوباره‌درصف‌گذاشتنِ یک retry به صف پر بخورد و job بدون تایمر جا بماند. این sweep عمداً jobهای *interrupted*
  را رد می‌کند: آن‌ها اعتبارنامهٔ خود را در ری‌استارت از دست داده‌اند، پس اجرایشان روی احراز هویت شکست می‌خورد،
  بودجهٔ retry را می‌سوزاند و به شکست نهایی می‌رسد — و همان مسیر آشتی‌دهی بالا را از بین می‌برد. حل آن‌ها با
  خودِ فراخوان‌کننده و از راه `get_operation_status` است.

اعتبارنامهٔ هر job تا وقتی امکان تلاش دوباره هست در حافظه نگه داشته می‌شود و از طریق callbackای به نام
`ports.JobSettled` آزاد می‌گردد که pool فقط پس از رسیدن job به وضعیت نهایی صدایش می‌زند — هرگز بین دو تلاش.

### Ports و Adapters (معماری شش‌ضلعی)

هسته صاحب اینترفیس‌هاست و آداپترها آن‌ها را پیاده می‌کنند. جهت وابستگی **فقط به سمت داخل** است:
`../internal/core` هیچ ایمپورتی از Fiber، `database/sql`، هیچ MCP SDK و هیچ کلاینت ارائه‌دهنده ندارد. بخش
[معماری](#sec-4) را ببینید.

---

## <a id="sec-3"></a>۳. ابزارها و تکنولوژی‌های استفاده‌شده

| موضوع | انتخاب | چرا |
| --- | --- | --- |
| زبان | **Go** (فایل `../go.mod` روی `go 1.26.2` قفل است) | باینری استاتیک، self-host آسان |
| HTTP | **Fiber v3** (نسخهٔ v3.5.0) | مبتنی بر fasthttp؛ پکیج `middleware/sse` نیمهٔ استریمِ ترنسپورت را سرو می‌کند |
| ذخیره‌سازی | **SQLite** با درایور **`modernc.org/sqlite`** | درایور خالص Go. درایورهای مبتنی بر cgo (مثل `mattn/go-sqlite3`) صراحتاً ممنوع‌اند تا بیلد استاتیک و کراس‌کامپایل/Docker ساده بماند |
| صف | کانال‌های Go + worker pool کران‌دار درون‌پروسه‌ای | بدون Redis، بدون بروکر، بدون سطح عملیاتی اضافه |
| Retry/Backoff | **`github.com/cenkalti/backoff/v4`** در صف؛ retry دست‌نویس در کلاینت ارائه‌دهنده | retry در سطح job و در سطح درخواست دو مسئلهٔ متفاوت‌اند |
| پیکربندی | متغیرهای محیطی + **`github.com/joho/godotenv`** | ۱۲-factor، با یک `.env` برای اجرای محلی |
| لاگ | `log/slog` با سطح قابل تنظیم | لاگ ساخت‌یافته، کتابخانهٔ استاندارد |
| شناسه‌ها | `crypto/rand`، ۱۲۸ بیت هگز | operation_id به کاربر داده می‌شود، پس نباید قابل حدس باشد |
| تست | `go test ./...` — همهٔ پکیج‌ها تست دارند | Race detector در CI روشن است |
| Lint | `go vet` + **golangci-lint v2.12** با فایل `../.golangci.yml` | به‌صورت job جداگانه در CI |
| کانتینر | `../Dockerfile` چندمرحله‌ای، `CGO_ENABLED=0`، ایمیج نهایی **distroless nonroot** | بدون شل، بدون libc، بدون root در ایمیج اجرا |
| CI/CD | GitHub Actions | بیلد/تست، لینت، ریلیز معنایی، انتشار ایمیج روی GHCR |
| ریلیز | **go-semantic-release** | بومیِ Go؛ لازم نیست فقط برای ریلیز، زنجیرهٔ ابزار Python یا Node وارد پروژه شود |

**هدف استقرار فقط self-hosted است:** کانتینر داکر، VPS یا یک باینری استاتیک.
**Vercel و پلتفرم‌های سرورلس صراحتاً هدف نیستند.** آن‌ها پروسهٔ ماندگار، worker pool در حافظه و فایل پایدار
SQLite ندارند — و تمام تصمیم‌های معماری اینجا بر فرض وجود این‌ها بنا شده است. کدی ننویسید که فرض سرورلس داشته
باشد (مثلاً اینکه `/tmp` باقی می‌ماند یا یک goroutine می‌تواند از عمر یک request بیشتر زنده بماند).

به همان اندازه عمدی: هیچ Redis، Postgres یا صف پیام مدیریت‌شده‌ای «محض احتیاط» اضافه نمی‌شود. پرهیز از همین
پیچیدگی عملیاتی، اصلِ هدف این طراحی است.

---

## <a id="sec-4"></a>۴. معماری

do0ps از **معماری شش‌ضلعی (Ports & Adapters)** پیروی می‌کند.

<div dir="ltr">

```
                          ┌──────────────────────────────────┐
   MCP client             │           internal/core          │
   (Claude / Codex)       │                                  │
        │                 │  domain/   pure types & rules    │
        ▼                 │  ports/    interfaces core owns  │
  ┌──────────┐  Bearer    │  app/      use cases             │
  │  Fiber   │  auth      │                                  │
  │ + auth   │────────────┼──► ProvisionServer   (long)      │
  └──────────┘            │    CreateSnapshot    (long)      │
        │                 │    RestoreVM         (long)      │
        ▼                 │    ProvisionLoadBalancer (long)  │
  ┌──────────┐            │    ~60 fast use cases            │
  │   mcp    │  primary   │    GetOperationStatus / Recovery │
  │ adapter  │────────────└────┬──────────┬──────────┬───────┘
  │ 147 tools│                 │          │          │
  └──────────┘         ports.Queue  ports.JobRepository
        │                        │          │   ports.ParspackProvider
   POST /mcp  (JSON-RPC)         ▼          ▼               │
   GET  /mcp  (SSE)         ┌────────┐ ┌────────┐    ┌──────▼──────┐
                            │ queue  │ │ sqlite │    │  parspack   │
                            │ pool   │ │ store  │    │  client     │
                            └────────┘ └────────┘    └─────────────┘
                                 secondary (driven) adapters
```

</div>

**قانون لایه‌بندی.** `../internal/core` هیچ ایمپورتی از Fiber، `database/sql`، MCP SDK یا کلاینت HTTP ارائه‌دهنده
ندارد. تنها به اینترفیس‌هایی وابسته است که خودش در `../internal/core/ports` تعریف می‌کند. آداپترها به هسته
وابسته‌اند و هسته هرگز آداپتری را ایمپورت نمی‌کند. اگر دیدید دارید چنین ایمپورتی را داخل هسته اضافه می‌کنید، آن
منطق جای دیگری دارد — یا هسته به یک port جدید نیاز دارد.

**فعلاً یک port به ازای هر ارائه‌دهنده.** هسته `ports.ParspackProvider` را تعریف می‌کند؛ یک اینترفیس بزرگ
(حدود ۱۵۷ متد) که دقیقاً همان عملیات موردنیازش را فهرست می‌کند. این عمداً یک `HostingProvider` عمومیِ مشترک
میان همهٔ ارائه‌دهندگان نیست؛ اینترفیس مشترک وقتی طراحی می‌شود که دو-سه ارائه‌دهنده هم‌پوشانی واقعی را نشان
دهند. در همین حال شکل دادهٔ دامنه (`Server`، `DNSRecord`، `CDNZone`، `SSLOrder` و…) یکدست نگه داشته می‌شود تا
آن یکپارچه‌سازیِ بعدی یک تغییر مکانیکی باشد نه بازنویسی مدل داده.

**اسکلت مشترک عملیات طولانی.** فایل `../internal/core/app/longop.go` ماشین‌آلات مشترک عملیات طولانی را نگه
می‌دارد — نگاشت اعتبارنامه‌های فقط-در-حافظه و تنظیمات polling — و `CreateSnapshot` و `RestoreVM` از آن استفاده
می‌کنند. `ProvisionServer` قدیمی‌تر از این helper است و هنوز نسخهٔ خودش را دارد.

**ریشهٔ ترکیب (Composition Root).** فایل `../cmd/server/main.go` تنها جایی است که اجازه دارد همهٔ پکیج‌ها را
هم‌زمان بشناسد. آداپترها را می‌سازد، از طریق portها به حدود ۷۰ Use Case تزریق می‌کند، چهار هندلر job را ثبت
می‌کند، recovery استارت را اجرا و Fiber را بالا می‌آورد. خاموشی با یک context مشتق از سیگنال کنترل می‌شود: اول
Fiber پذیرش درخواست را قطع می‌کند، بعد worker pool تخلیه می‌شود و در آخر دیتابیس بسته می‌شود.

---

## <a id="sec-5"></a>۵. ساختار مخزن

<div dir="ltr">

```
cmd/server/                    main.go — composition root (+ main_test.go, an end-to-end server test)

internal/config/               env-var loading + slog construction. Plain glue: no Fiber, no adapters, no core
internal/auth/                 Bearer token middleware, sits in front of the mcp adapter

internal/core/
  domain/                      Server, DNSRecord, CDNZone (+ 10 more cdn_*.go), SSL types, Job, Operation,
                                 ProviderCredentials, sentinel errors — plain Go types, no dependencies
  ports/                       Queue, JobRepository, ParspackProvider, Clock, IDGenerator
  app/                         ~70 use cases, one file per business operation, each with a _test.go;
                                 longop.go holds the shared long-operation scaffolding

internal/adapters/
  mcp/                         primary adapter — tools.go plus 13 cdn_*/ssl_*/vpc_*/snapshot_* tool files,
                                 JSON-RPC framing, SSE stream, Fiber routes
  sqlite/                      job store + migrations/0001_jobs.sql
  queue/                       bounded channel worker pool with backoff-driven job retry
  system/                      wall clock and ID generation
  providers/
    parspack/                  client.go (three API surfaces, auth, retry, error mapping) plus one file per
                                 capability area (vms, keys, firewalls, loadbalancers, snapshots, vpcs,
                                 reserved_ips, ssl, cdn*), each with tests

skills/parspack-infra/         the end-user Skill: orchestration guidance for the chatbot (§10.2)
docs/api-specs/                Parspack OpenAPI specs — reference material, treated as ground truth
.claude/skills/, .agents/skills/   vendored coding-agent skills (§10.1)
.github/workflows/             ci, release, docker-publish, jiffy, close-linked-issues
Dockerfile, docker-compose.yml, Makefile, .golangci.yml, .env.example
AGENTS.md                      the authoritative guide for agents/contributors
CLAUDE.md                      a pointer to AGENTS.md, so the two never drift
```

</div>

---

## <a id="sec-6"></a>۶. نصب و راه‌اندازی

پیکربندی از محیط خوانده می‌شود. فایل `.env` در پوشهٔ جاری اگر موجود باشد بارگذاری می‌شود و راه راحتِ اجرای
محلی است؛ اما اختیاری است، پس کانتینر و CI می‌توانند متغیرها را مستقیم بدهند.

### پیش‌نیازها

- **Go نسخهٔ ۱.۲۶.۲** — همان نسخه‌ای که `../go.mod` قفل کرده، CI با `go-version-file: go.mod` می‌خواند و
  مرحلهٔ builder در Dockerfile نام می‌برد. این سه را با هم ارتقا دهید
- بدون زنجیرهٔ ابزار cgo، بدون دیتابیس بیرونی، بدون بروکر پیام
- اختیاری: Docker و Compose، و `golangci-lint` نسخهٔ v2.12 برای لینت

### ساخت توکن دسترسی

توکن‌ها باید حداقل **۱۶ کاراکتر** باشند؛ در غیر این صورت سرور بالا نمی‌آید.

<div dir="ltr">

```bash
openssl rand -hex 32
```

</div>

### شروع سریع: go run یا باینری

<div dir="ltr">

```bash
git clone https://github.com/javadib/do0ps.git
cd do0ps
git checkout develop

cp .env.example .env
# edit .env and set MCP_AUTH_TOKENS, e.g.
#   MCP_AUTH_TOKENS="<32-hex-token>:client-a:Ops Team"

make run          # go run ./cmd/server
```

</div>

دستور `make build` همه‌چیز را کامپایل می‌کند؛ برای گرفتن باینریِ قابل کپی روی VPS صریحاً بیلد کنید:

<div dir="ltr">

```bash
go build -o do0ps ./cmd/server
```

</div>

روی هاست مقصد با همان متغیرهای محیطی اجرایش کنید — چه از فایل `.env` و چه export مستقیم. دستورهای
`make test`، `make vet` و
`make lint` به‌ترتیب مجموعه تست، `go vet` و golangci-lint را اجرا می‌کنند؛ `make install-tools` هم دقیقاً همان
نسخهٔ golangci-lint که CI استفاده می‌کند را نصب می‌کند.

هنگام استارت خطی شبیه این چاپ می‌شود:

<div dir="ltr">

```
level=INFO msg=listening addr=:8080 tools=147 version=dev
```

</div>

### شروع سریع: Docker Compose

<div dir="ltr">

```bash
cp .env.example .env
# edit .env — at minimum set MCP_AUTH_TOKENS
docker compose up -d --build
```

</div>

این دستور تارگت `runner` را از `../Dockerfile` موجود می‌سازد و do0ps را روی `HTTP_PORT` (پیش‌فرض ۸۰۸۰) بالا
می‌آورد، با انبارهٔ job روی volume نام‌دار `do0ps-data`. از دست دادن آن volume یعنی از دست دادن تاریخچهٔ jobها،
پس مثل هر کانتینر stateful دیگری از آن پشتیبان بگیرید.

ایمیج نهایی `gcr.io/distroless/static-debian12:nonroot` است — بدون شل، بدون پکیج‌منیجر، با uid 65532. به همین
دلیل `../docker-compose.yml` هیچ healthcheck داخل کانتینر تعریف نکرده: داخل ایمیج نه `curl` هست نه `wget`. به‌جای
آن `GET /healthz` را از بیرون صدا بزنید.

فایل `.env` در `../.dockerignore` است، پس رمزهای شما هرگز داخل ایمیج نمی‌روند؛ Compose به‌جای آن متغیرها را از
طریق `env_file` تزریق می‌کند.

### بررسی سلامت

<div dir="ltr">

```bash
export TOKEN=<the token half of your MCP_AUTH_TOKENS entry>

# Liveness — deliberately outside the token allow-list, so orchestrators can probe it
curl -s localhost:8080/healthz
# {"status":"ok"}

# Tool discovery — requires a valid bearer token
curl -s localhost:8080/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Smoke-test the transport end to end with the built-in no-op tool
curl -s localhost:8080/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"ping","arguments":{}}}'
```

</div>

توکن غایب یا ناشناخته پاسخ `401` با بدنه‌ای عمداً بی‌اطلاع می‌گیرد — سرور هرگز اشاره نمی‌کند که *چرا* توکن رد
شده است.

---

## <a id="sec-7"></a>۷. مرجع پیکربندی

تمام پیکربندی از متغیرهای محیطی خوانده می‌شود و `.env` هنگام استارت بارگذاری می‌گردد. فایل
[`.env.example`](../.env.example) مرجع اصلیِ پیش‌فرض‌هاست؛ جدول زیر خلاصهٔ آن است.

| متغیر | الزامی | پیش‌فرض | معنا |
| --- | --- | --- | --- |
| `MCP_AUTH_TOKENS` | برای `http` **بله** | — | فهرست مجاز Bearer: `token:client_id[:name]` با جداکنندهٔ کاما. توکن حداقل ۱۶ کاراکتر. در حالت `stdio` استفاده نمی‌شود، چون listenerی برای محافظت وجود ندارد |
| `MCP_TRANSPORT` | خیر | `http` | `http` (‏Streamable HTTP روی Fiber) یا `stdio` (کلاینت چت خودش این باینری را اجرا می‌کند؛ همان چیزی که یک MCP Bundle نصب‌شده با آن کار می‌کند). فلگ `--stdio` هم همین کار را می‌کند |
| `DO0PS_SERVER_URL` | خیر | — | فقط برای bundle: نشانی endpoint یک سرور do0ps که این پروسه به آن پل می‌زند، مثل `https://do0ps.example.com/mcp`. خالی یعنی سرور درون همین پروسه اجرا شود. در حالت `http` پذیرفته نمی‌شود |
| `DO0PS_AUTH_TOKEN` | خیر | — | توکن Bearer برای `DO0PS_SERVER_URL`. هر وقت آن تنظیم شود، این هم لازم است |
| `DB_PATH` | خیر | `../data/do0ps.db` | فایل SQLite انبارهٔ job. پوشهٔ والدش خودکار ساخته می‌شود. در حالت `stdio` پیش‌فرض به پوشهٔ config کاربر منتقل می‌شود |
| `HTTP_PORT` | خیر | `8080` | پورتی که سرور روی آن گوش می‌دهد |
| `LOG_LEVEL` | خیر | `info` | یکی از `debug`، `info`، `warn` یا `error` |
| `DO0PS_QUEUE_WORKERS` | خیر | `8` | تعداد goroutineهای worker |
| `DO0PS_QUEUE_DEPTH` | خیر | `256` | عمق کران‌دار کانال کار. وقتی پر باشد کار رد می‌شود، نه اینکه بی‌حد بافر شود |
| `DO0PS_POLL_INTERVAL` | خیر | `10s` | فاصلهٔ poll کردن ارائه‌دهنده در عملیات طولانی (قالب duration در Go) |
| `DO0PS_POLL_TIMEOUT` | خیر | `20m` | سقف زمان یک job طولانی پیش از آنکه failed ثبت شود |
| `DO0PS_SHUTDOWN_WAIT` | خیر | `30s` | مدتی که خاموشی graceful منتظر تخلیهٔ jobهای در جریان می‌ماند |

نمونه:

<div dir="ltr">

```
MCP_AUTH_TOKENS="change-me-0123456789:client-a,change-me-9876543210:client-b"
```

</div>

**`client_id` رزرو شده است.** امروز پارس می‌شود و به هر درخواست احرازشده می‌چسبد اما جای دیگری استفاده نمی‌شود؛
وجودش برای این است که حالت چندمستأجری (multi-tenant) در آینده نیاز به بازنویسی لایهٔ احراز هویت نداشته باشد.
جدول `jobs` هم به همین دلیل ستون بلااستفادهٔ `tenant_id` دارد.

---

## <a id="sec-8"></a>۸. رابط MCP

### ترنسپورت

Streamable HTTP روی Fiber، روی یک مسیر، با هر دو نیمه پیاده‌سازی‌شده:

| مسیر | احراز هویت | کاربرد |
| --- | --- | --- |
| `GET /healthz` | ندارد | بررسی liveness |
| `POST /mcp` | Bearer | درخواست/پاسخ JSON-RPC 2.0 |
| `GET /mcp` | Bearer | استریم SSE برای پیام‌های آغازشده از سمت سرور |

نیمهٔ SSE یک رویداد `ready` می‌فرستد (`{"protocol":"mcp-streamable-http"}`) و بعد اتصال را باز نگه می‌دارد و با
`stream.Context()` قطع‌شدن کلاینت را می‌فهمد. هنوز هیچ Use Caseای چیزی داخل این استریم push نمی‌کند — فعلاً
فقط رفت‌وبرگشت ترنسپورت را ثابت می‌کند.

متدهای JSON-RPC که فعلاً پاسخ داده می‌شوند: **`tools/list`** و **`tools/call`**. هر چیز دیگری `-32601` می‌گیرد.

### نگاشت خطاها

خطاها از sentinelهای دامنه به کدهای JSON-RPC ترجمه می‌شوند تا فراخوان‌کننده چیز قابل‌استفاده‌ای بگیرد و در عین
حال جزئیات داخلی بیرون نرود:

| شرایط | کد | بدنه |
| --- | --- | --- |
| JSON-RPC نامعتبر | `-32600` | `malformed JSON-RPC request` (HTTP 400) |
| متد یا ابزار ناشناخته | `-32601` / `-32602` | `unsupported method X` / `unknown tool X` |
| ورودی نامعتبر، اعتبارنامهٔ اشتباه، یافت‌نشدن منبع | `-32602` | متن دقیق خطا |
| هر چیز دیگر | `-32603` | `tool X failed` — خطای کامل فقط در لاگ سرور ثبت می‌شود، نه در پاسخ |

### فهرست ابزارها

**۱۴۷ ابزار** ثبت شده است. هر ابزاری که با ارائه‌دهنده کار می‌کند `api_key` و — در سطوحی که جفت‌کلید دارند —
`secret_key` می‌پذیرد.

| خانواده | تعداد | نمونه‌ها |
| --- | --- | --- |
| موتورهای Rule در CDN | ۱۷ | `create_cdn_origin_rule`، `update_cdn_page_rule`، `toggle_cdn_transform_rule` |
| ModSec WAF در CDN | ۱۲ | `update_cdn_modsec_status`، `create_cdn_modsec_rule` |
| Load Balancing لبه (CDN) | ۱۰ | `create_cdn_load_balance`، `update_cdn_load_balance_server` |
| تنظیمات زون CDN | ۱۰ | `update_cdn_antivirus_status`، `update_cdn_developer_mode`، `update_cdn_dnssec_status` |
| فایروال لبه (CDN) | ۹ | `create_cdn_access_rule`، `update_cdn_ip_reputation`، `update_cdn_ddos_actions` |
| لاگ و آنالیتیکس CDN | ۸ | `get_cdn_access_log`، `get_cdn_waf_log`، `get_cdn_monthly_traffic_usage` |
| تنظیمات شبکهٔ CDN | ۸ | `update_cdn_https_convertor`، `update_cdn_web_socket`، `update_cdn_www_redirection` |
| سفارش گواهی SSL | ۸ | `create_ssl_order`، `process_ssl_order`، `verify_ssl_challenge`، `reissue_ssl_certificate` |
| کش CDN | ۷ | `purge_cdn_cache`، `update_cdn_cache_ttl`، `list_cdn_cache_entries` |
| Bulklistهای CDN | ۶ | `create_cdn_bulklist`، `list_cdn_firewall_countries` |
| محدودسازی نرخ در CDN | ۶ | `create_cdn_rate_limit_rule`، `update_cdn_rate_limit_rule_priority` |
| زون‌های CDN | ۵ | `create_cdn_zone`، `list_cdn_zones`، `list_cdn_plans` |
| SSL در سطح زون CDN | ۵ | `update_cdn_min_tls_version`، `update_cdn_hsts`، `list_cdn_certificates` |
| رکوردهای DNS | ۵ | `create_dns_record`، `update_dns_record`، `get_nameserver_records` |
| فایروال VM | ۵ | `create_firewall`، `update_firewall` |
| Load Balancer در سطح VM | ۵ | `create_load_balancer` (طولانی)، `update_load_balancer` |
| IPهای رزرو | ۴ | `reserve_ip`، `assign_ip_to_server`، `release_ip` |
| اسنپ‌شات و بازیابی | ۴ | `create_snapshot` (طولانی)، `restore_vm` (طولانی) |
| چرخهٔ عمر VM | ۴ | `create_server` (طولانی)، `list_servers`، `get_server`، `delete_server` |
| VPCها | ۴ | `create_vpc`، `list_vpcs` |
| کلیدهای SSH | ۳ | `register_ssh_key`، `list_ssh_keys` |
| عملیات | ۱ | `get_operation_status` |
| تست دود ترنسپورت | ۱ | `ping` — ابزار داخلی بدون Use Case و بدون ارائه‌دهنده |

دو خانوادهٔ فایروال و دو خانوادهٔ Load Balancer وجود دارد و **نباید با هم اشتباه گرفته شوند**: ابزارهای
`*_firewall` و `*_load_balancer` روی شبکهٔ cloud-server (مبتنی بر Abrha) کار می‌کنند، در حالی که معادل‌های
`*_cdn_*` روی لبهٔ CDN عمل می‌کنند. این‌ها روی دو سطح API متفاوت هستند.

### نمونه: یک عملیات طولانی از ابتدا تا انتها

<div dir="ltr">

```bash
TOKEN=<your do0ps token>

# 1. Start it — returns immediately
curl -s localhost:8080/mcp -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{
  "jsonrpc":"2.0","id":1,"method":"tools/call",
  "params":{"name":"create_server","arguments":{
    "api_key":"<provider key>","name":"web-01","region":"tehran",
    "image":"ubuntu-24.04","cpu_cores":2,"ram_mb":2048,"disk_gb":40}}}'

# 2. Poll it. Passing api_key lets this call reconcile with the provider
#    if the server restarted while the operation was in flight.
curl -s localhost:8080/mcp -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{
  "jsonrpc":"2.0","id":2,"method":"tools/call",
  "params":{"name":"get_operation_status","arguments":{
    "operation_id":"<id from step 1>","api_key":"<provider key>"}}}'
```

</div>

---

## <a id="sec-9"></a>۹. اتصال کلاینت MCP

دو راه برای اتصال وجود دارد و هر دو همان ۱۴۷ ابزار را ارائه می‌دهند، چون هر دو transport از یک dispatcher
مشترک عبور می‌کنند.

### گزینهٔ الف: نصب MCP Bundle (بدون نیاز به اجرای سرور)

فایل `.mcpb` مربوط به سیستم‌عامل خود را از [Releases](https://github.com/javadib/do0ps/releases) دانلود و در
کلاینت چت نصب کنید — در Claude Desktop کافی است روی آن دابل‌کلیک کنید یا در **Settings → Extensions** رهایش
کنید. کلاینت، باینری داخل bundle را روی stdio اجرا و از آن پس خودش مدیریتش می‌کند.

تنظیمات این extension دو فیلد اختیاری دارد. اگر هر دو را خالی بگذارید، bundle خودش کل سرور را اجرا می‌کند —
بدون host، بدون port و بدون توکن. اگر **نشانی سرور** و **توکن دسترسی** را پر کنید، همان bundle به یک پل نازک
به سمت سرور self-hosted شما تبدیل می‌شود، تا یک تیم روی یک استقرار و یک تاریخچهٔ job مشترک باشد.

برای کلاینت‌هایی که فایل `.mcpb` نمی‌خوانند، bundle را unzip کنید و کلاینت را به `server/do0ps --stdio` وصل
کنید. پیکربندی هر کلاینت و دستور build در [docs/mcp-bundle.md](mcp-bundle.md) آمده است.

### گزینهٔ ب: اتصال به سرور self-hosted

کلاینت خود را با یکی از توکن‌های `MCP_AUTH_TOKENS` به مسیر `/mcp` وصل کنید:

<div dir="ltr">

```json
{
  "mcpServers": {
    "do0ps": {
      "type": "http",
      "url": "https://your-host.example.com/mcp",
      "headers": { "Authorization": "Bearer <your do0ps token>" }
    }
  }
}
```

</div>

این را در پیکربندی MCP کلاینت خود بگذارید (برای Claude Code یک `.mcp.json` در پروژه یا کانفیگ سطح کاربر؛ برای
Claude Desktop فایل `claude_desktop_config.json`). حتماً TLS را جلوی سرور terminate کنید — توکن یک اعتبارنامهٔ
bearer است و نباید از روی اتصال رمزنشده عبور کند.

اعتبارنامهٔ ارائه‌دهنده اینجا پیکربندی **نمی‌شود**؛ نشست چت‌بات آن را در هر فراخوانی ابزار می‌فرستد.

---

## <a id="sec-10"></a>۱۰. Skill‌ها

واژهٔ «Skill» در این پروژه دو معنای متفاوت دارد. آن‌ها را با هم اشتباه نگیرید.

### ۱۰.۱ Skillهای عاملِ کدنویس — برای کسانی که do0ps را می‌سازند

مخزن دو Skill مربوط به Go را از [`samber/cc-skills-golang`](https://github.com/samber/cc-skills-golang) داخل
خودش نگه می‌دارد تا هر عاملی که روی این کدبیس کار می‌کند Go یکدست بنویسد:

| Skill | چه چیزی را پوشش می‌دهد |
| --- | --- |
| `golang-code-style` | طول خط و شکستن آن، اعلان متغیر، شفافیت کنترل جریان، اینکه کجا کامنت کمک می‌کند و کجا ضرر می‌زند |
| `golang-design-patterns` | functional options، طراحی سازنده‌ها، جریان خطا، چرخهٔ عمر منابع، graceful shutdown، تاب‌آوری، تزریق وابستگی، مراجع معماری hexagonal/clean |

این‌ها دو بار در مخزن ثبت شده‌اند، در `../.claude/skills` و `../.agents/skills`، تا هم Claude Code و هم سایر
ابزارهای عامل، هرکدام از مسیر مرسوم خودشان آن‌ها را بردارند. فایل `../skills-lock.json` در ریشهٔ مخزن هرکدام را با
مخزن مبدأ، مسیر و هش محتوا قفل می‌کند — همین باعث می‌شود هر به‌روزرسانی در diff دیده شود، نه اینکه بی‌صدا اتفاق
بیفتد.

رد پای این Skillها در سراسر کدبیس پیداست: functional options با اعتبارسنجی زودهنگام (`WithWorkers`،
`WithPollTimeout` و `WithCDNBaseURL` همه هنگام ساخت شکست می‌خورند نه زیر بار)، سازنده‌هایی که `(T, error)`
برمی‌گردانند، خطاهای پیچیده‌شده با `%w` و مسیر خاموشی‌ای که workerها را زیر یک context کران‌دار تخلیه می‌کند.

### ۱۰.۲ Skill کاربر نهایی — برای چت‌باتی که از do0ps استفاده می‌کند

فایل [`../skills/parspack-infra/SKILL.md`](../skills/parspack-infra/SKILL.md) همان آرتیفکتی است که به دستیارِ
*مصرف‌کننده* می‌گوید چطور رفتار کند. برای مدلی نوشته شده که با کاربر غیرفنی حرف می‌زند و این‌ها را پوشش می‌دهد:

- جدول ابزارها با علامت‌گذاری **سریع** یا **طولانی** بودن هرکدام
- قواعدی که برای هر فراخوانی صدق می‌کند (هرگز خروجی خام ابزار را جلوی کاربر نریز؛ خلاصه کن)
- الگوی `operation_id` و polling برای عملیات طولانی
- گردش‌کار هر حوزه: ساخت سرور، کلیدهای SSH، زون‌های CDN و رکوردهای DNS، فایروال‌ها، Load Balancerها، IPهای
  رزرو، VPCها، اسنپ‌شات و بازیابی، گواهی‌های SSL
- نحوهٔ انتقال خطاها به کاربر

این عمداً **همان AGENTS.md نیست**: فایل AGENTS.md برای آدم‌ها و عامل‌هایی است که این پروژه را *می‌سازند*، در
حالی که Skill رفتار چت‌بات رو به کاربر نهایی را تعیین می‌کند.

توجه کنید که این Skill حدود ۵۰ ابزار — هستهٔ VM/DNS/SSL — را مستند می‌کند و برای پوشش خانواده‌های لبهٔ CDN که
بعداً اضافه شدند به‌روز نشده است، پس چت‌باتی که آن را بارگذاری می‌کند برای حدود ۹۰ ابزار CDN راهنمایی ندارد.

---

## <a id="sec-11"></a>۱۱. MCPهای متصل

### ۱۱.۱ سرور MCP که خود این پروژه منتشر می‌کند

do0ps خودش یک سرور MCP است — ۱۴۷ ابزارش در [رابط MCP](#sec-8) و نحوهٔ وصل‌کردنش به چت‌بات در
[اتصال کلاینت MCP](#sec-9) آمده است.

### ۱۱.۲ سرورهای MCP که در توسعهٔ do0ps استفاده می‌شوند

هیچ فایل `.mcp.json` در مخزن ثبت نشده است، بنابراین کلون‌کردن این مخزن چیزی را خودکار وصل نمی‌کند — این‌ها به
ازای هر توسعه‌دهنده یا هر محیط عامل پیکربندی می‌شوند. مواردی که در گردش‌کار عاملِ این پروژه استفاده می‌شوند:

| سرور | برای چه |
| --- | --- |
| **GitHub MCP** | خواندن و نوشتن ایشیوها، Pull Requestها، ریویوها، برنچ‌ها و وضعیت CI. گردش‌کار ایشیومحورِ بخش ۱۳ عملاً از همین راه اجرا می‌شود |
| **Notion MCP** | یادداشت‌ها و صفحات مستندات پروژه که بیرون از مخزن نگه‌داری می‌شوند |
| **Google Drive MCP** | اسناد و اسپک‌های ارائه‌دهنده که مالک پروژه به اشتراک گذاشته است |

اگر همین چیدمان را می‌خواهید، آن‌ها را به پیکربندی کلاینت خودتان اضافه کنید؛ هیچ بخشی از بیلد به آن‌ها وابسته
نیست.

### ۱۱.۳ جیفی (Jiffy)

ورک‌فلوی `../.github/workflows/jiffy.yml` هر ایشیو یا کامنتی که `@jiffy` را ذکر کند به gateway خودمیزبانِ جیفی
می‌فرستد و آن‌جا رشتهٔ گفت‌وگو به یک Pull Request تبدیل می‌شود. این مسیر با متغیر مخزنِ `JIFFY_USER_WHITELIST`
کنترل می‌شود — اگر whitelist تنظیم نشده باشد، ورک‌فلو به‌جای ارسال، دستور راه‌اندازی را روی ایشیو کامنت می‌کند و
متوقف می‌شود.

---

## <a id="sec-12"></a>۱۲. APIهای ارائه‌دهنده (پارس‌پک)

پارس‌پک **سه سطح API جداگانه** دارد — همان هاست، همان طرح احراز هویت Bearer، اما پیشوند مسیرِ متفاوت. کلاینت
هر سه آدرس پایه را جدا نگه می‌دارد و هرکدام برای تست قابل جایگزینی است:

| سطح | آدرس پایه | آپشن کلاینت | اسپک در این مخزن |
| --- | --- | --- | --- |
| Cloud Server (ماشین/شبکه، مبتنی بر Abrha) | `https://my.parspack.com/cserver` | `WithBaseURL` | ثبت نشده — با `github.com/abrhacom/go-api-abrha` تطبیق بدهید |
| CDN (زون‌ها — **رکوردهای DNS اینجا هستند**) | `https://my.parspack.com/cdnapi` | `WithCDNBaseURL` | `api-specs/parspack-cdn.openapi.yaml` |
| SSL (گردش‌کار سفارش گواهی) | `https://my.parspack.com/sslv2` | `WithSSLBaseURL` | `api-specs/parspack-ssl.openapi.yaml` |

دو فایل OpenAPI ثبت‌شده مستقیماً از مالک پروژه گرفته شده‌اند و **مرجع معتبر** به حساب می‌آیند — آن‌ها را به
استخراج دوبارهٔ شکل endpointها از `docs.parspack.com` ترجیح دهید؛ آن سایت یک SPA رندرشده با JS است که ابزارها
معمولاً نمی‌توانند اسکرپش کنند.

**DNS در پارس‌پک محصول مستقلی نیست** (همین موضوع دربارهٔ ابرآروان هم صدق می‌کند): رکوردهای DNS *داخل* پیکربندی
زون CDN و ذیل یک `zone_uuid` مدیریت می‌شوند، نه از طریق یک API جداگانهٔ DNS. به همین دلیل چیزی به نام
`ports.ParspackDNS` وجود ندارد — عملیات DNS کنار قابلیت CDN روی همان `ParspackProvider` می‌نشیند.

ارائه‌دهندگان بعدی پس از پارس‌پک: **ابرآروان (ArvanCloud)** و **لیارا (Liara)**.

---

## <a id="sec-13"></a>۱۳. روند توسعه

### مدل برنچ‌ها

برنچ پیش‌فرض مخزن `master` است، اما **کار روی `develop` توسعه و مرج می‌شود**. برنچ خود را از `develop` بزنید
و PR را هم به `develop` بدهید؛ `master` جایی است که ریلیزهای پایدار از آن بریده می‌شود.

این یک پیامد مهم دارد: بستن خودکار ایشیو با «Closes #N» در گیت‌هاب فقط برای برنچ پیش‌فرض کار می‌کند، بنابراین
ورک‌فلوی `../.github/workflows/close-linked-issues.yml` همان اسکنِ کلیدواژه را برای مرج در برنچ‌های غیرپیش‌فرض
دوباره پیاده کرده است. بدون آن، ایشیویی که روی `develop` مرج می‌شود باز و روی `status:in-progress` می‌ماند و
بی‌صدا هر ایشیوی وابسته به آن را قفل می‌کند.

### قراردادها

- جهت وابستگی یک‌طرفه است: `adapters → core` و هرگز برعکس
- با `gofmt`/`goimports` فرمت کنید (فایل `../.golangci.yml` پیشوند ایمپورت محلی را تنظیم کرده)؛ `go vet` و
  golangci-lint را تمیز نگه دارید
- خطاها را با زمینه بپیچید: `fmt.Errorf("doing X: %w", err)`. در کد کتابخانه‌ای از panic بپرهیزید — خطا برگردانید
- **تمام کامنت‌ها، خروجی لاگ و شناسه‌ها باید انگلیسی باشند**، صرف‌نظر از زبانی که در ایشیوها و بحث‌های پروژه
  استفاده می‌شود
- هرگز هیچ رمز یا اعتبارنامه‌ای در مخزن کامیت نمی‌شود. اعتبارنامهٔ ارائه‌دهنده فقط در زمان اجرا و از طریق
  پارامترهای فراخوانی ابزار MCP جریان دارد

### کار از روی ایشیوهای گیت‌هاب

کل کار به شکل ایشیو روی `javadib/do0ps` دنبال می‌شود. هر ایشیو دو دستهٔ برچسب دارد، به‌علاوهٔ `backlog` برای
هر چیزی که عمداً به تعویق افتاده:

- `area:*` — یکی از `core`، `sqlite`، `queue`، `auth`، `mcp`، `parspack`، `infra`، `docs`، `research`
- `status:*` — `ready` (شروعش امن است)، `in-progress` (**یکی دارد رویش کار می‌کند — شروع نکنید**)،
  `blocked` (وابستگیِ برآورده‌نشده‌ای دارد که در متن ایشیو ذکر شده)

**پیش از نوشتن هر خط کد برای یک ایشیو، برچسبش را از `status:ready` به `status:in-progress` تغییر دهید.** این
یک قانون سخت است: تنها چیزی است که جلوی دوباره‌کاریِ دو عامل را که از یک صف ایشیوی مشترک برمی‌دارند می‌گیرد.
وابستگی‌هایی که به شکل «Depends on #4» ذکر شده‌اند پیش‌نیاز قطعی‌اند — به‌جای اینکه از شمارهٔ ایشیو نتیجه بگیرید
انجام شده، مطمئن شوید واقعاً بسته شده است.

### CI

| ورک‌فلو | تریگر | چه می‌کند |
| --- | --- | --- |
| `ci.yml` | PR یا push روی `master` یا `develop` | `go build`، `go vet`، `go test ./... -race -cover`؛ golangci-lint v2.12 به‌عنوان job موازی |
| `release.yml` | push روی `master` | go-semantic-release نسخهٔ `vX.Y.Z` می‌زند و سپس ورک‌فلوهای داکر و mcpb را صدا می‌زند |
| `release.yml` | push روی `develop` | نسخهٔ بعدی را حساب می‌کند و آن را به‌صورت pre-release با تگ `vX.Y.Z-RC.N` منتشر می‌کند. بدون ایمیج و بدون bundle |
| `docker-publish.yml` | تگ `v*.*.*`، دستی، یا فراخوانی از ریلیز | تارگت `runner` را با `APP_VERSION` می‌سازد و به GHCR پوش می‌کند |
| `jiffy.yml` | باز شدن ایشیو یا کامنتی حاوی `@jiffy` | رشتهٔ گفت‌وگوی ایشیو را به gateway جیفی می‌فرستد |
| `close-linked-issues.yml` | مرج PR در یک برنچ غیرپیش‌فرض | ایشیوهای ارجاع‌شده با کلیدواژه‌های بستن را می‌بندد |

### نسخه‌گذاری و ریلیز

فقط `master` نسخهٔ پایدار تولید می‌کند. `develop` برای نسخه‌ای که در صف است release candidate می‌سازد و
چیز دیگری منتشر نمی‌کند.

| برنچ | تولید می‌کند | pre-release | ایمیج داکر | ‏MCP bundle |
| --- | --- | --- | --- | --- |
| `master` | `vX.Y.Z` | خیر | بله | بله |
| `develop` | `vX.Y.Z-RC.N` | بله | خیر | خیر |

تا پیش از ۱.۰ نسخه‌ها در ۰.x می‌مانند: از پایهٔ 0.0.0 اولین ریلیز `v0.1.0` است و یک تغییر breaking به‌جای
پریدن به `v1.0.0` فقط minor را بالا می‌برد. این کار `allow-initial-development-versions` است و
go-semantic-release **به‌محض وجود یک ریلیز با major بزرگ‌تر یا مساوی ۱ آن را نادیده می‌گیرد** — پس برای
فعال شدن نسخه‌گذاری ۰.x باید ریلیز فعلی `v1.0.0` حذف شود.

پسوند `-RC.N` را خود ورک‌فلو حساب می‌کند نه go-semantic-release، چون این ابزار توان ساختنش را ندارد: ورودی
`prerelease` آن فقط تیکِ pre-release در گیت‌هاب را می‌زند و نسخه را تغییر نمی‌دهد، و مسیر
`--maintained-version` هم پایه‌اش را هر بار از روی همان فلگ بازمی‌سازد، پس همیشه `RC.1` برمی‌گرداند و push
دوم با تگ اول تصادم می‌کند. در عوض `develop` با `--dry` نسخهٔ بعدی را از semantic-release می‌پرسد و سپس
اولین شمارهٔ `RC` آزاد را از روی تگ‌های موجود برمی‌دارد — چون چیزی ذخیره نمی‌شود، شمارنده در برابر اجرای
دوباره و revert هم درست می‌ماند.

### تست

<div dir="ltr">

```bash
make test          # go test ./...
go test ./... -race -cover
```

</div>

حالا همهٔ پکیج‌ها تست دارند، از جمله یک تست سرتاسری در `../cmd/server/main_test.go` و تست آداپتر برای هر حوزهٔ
قابلیت پارس‌پک. **اما یک پکیج در حال حاضر fail می‌شود** — پایین‌تر ببینید.

---

## <a id="sec-14"></a>۱۴. وضعیت فعلی پروژه و کاستی‌های شناخته‌شده

| بخش | وضعیت |
| --- | --- |
| ساختار شش‌ضلعی، دامنه، portها | پیاده‌سازی شده |
| حدود ۷۰ Use Case در حوزهٔ VM، شبکه، CDN و SSL | پیاده‌سازی و یونیت‌تست شده |
| انبارهٔ job در SQLite + migration + recovery هنگام استارت | پیاده‌سازی و تست شده |
| Worker pool: کران‌دار، تخلیهٔ graceful، retry با backoff (حداکثر ۵ تلاش)، sweep بازیابی روی `ListDue` | پیاده‌سازی و تست شده |
| میان‌افزار احراز هویت Bearer (فهرست مجاز هش‌شده، مقایسهٔ زمان‌ثابت) | پیاده‌سازی و تست شده |
| رجیستری ابزار MCP: ۱۴۷ ابزار، `tools/list` و `tools/call` | پیاده‌سازی و تست شده |
| Streamable HTTP: هم POST و هم SSE روی `GET /mcp` | پیاده‌سازی و تست شده |
| ‏transport روی stdio (`--stdio`) برای bundleهای نصب‌شده | پیاده‌سازی و تست شده |
| آداپتر پارس‌پک روی هر سه سطح API | پیاده‌سازی و در برابر سرورهای ساختگی تست شده |
| Dockerfile (distroless، nonroot، استاتیک) + docker-compose | کامیت شده |
| Makefile، `../.golangci.yml`، `.env.example` | کامیت شده |
| Skill کاربر نهایی (`../skills/parspack-infra`) | هر ۱۴۷ ابزار را پوشش می‌دهد |
| هندشیک `initialize` و `ping` در MCP | روی هر دو transport پیاده شده |
| ‏transport روی stdio و ساخت bundle با فرمت `.mcpb` | پیاده شده، در CI اسموک‌تست می‌شود |
| LICENSE | با وجود قصد متن‌باز بودن، هنوز کامیت نشده است |

### مواردی که به‌تازگی رفع شدند

موارد زیر روی همین برنچ اصلاح شده‌اند و اینجا ثبت می‌شوند تا ردیابیِ تغییر ساده باشد:

- **الزامی بودن `.env`.** تابع `config.Load()` متد `godotenv.Load()` را صدا می‌زد و در نبود فایل `log.Fatal`
  می‌کرد؛ نتیجه‌اش شکست `go test ./...`، قرمز ماندن CI و بالا نیامدنِ اصلاً ایمیج داکر بود (فایل `.env` در
  `../.dockerignore` است، پس ایمیج هرگز نمی‌تواند آن را داشته باشد). حالا بارگذاری best-effort است
  (`_ = godotenv.Load()`) و همان بررسی‌های الزامیِ بعدی تصمیم می‌گیرند پیکربندی قابل استفاده هست یا نه.
- **ناهماهنگی نسخهٔ Go.** فایل `../AGENTS.md` می‌گفت 1.25+ و Dockerfile روی تگ شناورِ `golang:1.26-alpine` بیلد
  می‌کرد. هر دو حالا **1.26.2** را نام می‌برند، هماهنگ با `../go.mod`.
- **`godotenv` به‌عنوان indirect علامت خورده بود** در `../go.mod`، با اینکه مستقیم ایمپورت می‌شد —
  `go mod tidy` جابه‌جایش کرد.
- **کامنت هدف `run` در `../Makefile`** نام `DO0PS_TOKENS` را می‌برد؛ متغیر درست `MCP_AUTH_TOKENS` است.
- **Skill کاربر نهایی فقط حدود ۵۰ ابزار اولیه را پوشش می‌داد.** حالا هر ۱۴۷ ابزار را مستند می‌کند، به‌همراه
  خانواده‌های لبهٔ CDN و یک بخش گردش‌کار برای آن‌ها.
- **retry هرگز نمی‌توانست احراز هویت کند.** هر هندلر عملیات طولانی اعتبارنامهٔ فراخوان‌کننده را با یک `defer`
  در همان تلاش اول آزاد می‌کرد؛ نتیجه اینکه تلاش دوم با `ErrInvalidCredentials` شکست می‌خورد، تلاش‌های سوم تا
  پنجم هم همین‌طور، و job نهایتاً `failed` می‌شد — در حالی که provider دقیقاً یک بار صدا زده شده بود و مسیر
  آشتی‌دهی هم بسته می‌شد. حالا اعتبارنامه تا نهایی‌شدن job زنده می‌ماند، از طریق `ports.JobSettled`.
- **متد `JobRepository.ListDue` فراخوان‌کننده‌ای نداشت.** حالا sweep بازیابی از آن استفاده می‌کند (بخش
  [مفاهیم](#sec-2))، با یک محافظ صریح که سراغ jobهای interrupted نرود.

---

## <a id="sec-15"></a>۱۵. نقشه راه

هر چیزی که ایشیوتراکر برای انتشار اول تعریف کرده بود تحویل شده است: دامنه و portهای هسته، انبارهٔ job روی
SQLite به‌همراه recovery، worker pool صف، احراز هویت Bearer، ترنسپورت MCP، آداپترهای پارس‌پک روی هر سه سطح
API، ریشهٔ ترکیب، بسته‌بندی داکر، ‏MCP bundle و Skill کاربر نهایی.

قدم‌های بعدی:

| حوزه | کار |
| --- | --- |
| ارائه‌دهنده‌های بیشتر | آداپترهای ابرآروان و لیارا، هرکدام پشت port اختصاصی خودش |
| یکسان‌سازی port | وقتی دو-سه ارائه‌دهنده وجود داشت و هم‌پوشانی واقعی دیده شد، portهای تک‌ارائه‌دهنده با یک port مشترک جایگزین شوند. خودِ `ports.ParspackProvider` با حدود ۱۵۷ متد دلیل خوبی است که هنگام آن refactor به تفکیک حوزهٔ قابلیت شکسته شود |
| پوشش Skill | گسترش Skill کاربر نهایی از هستهٔ VM/DNS/SSL به حدود ۹۰ ابزار لبهٔ CDN |
| LICENSE | با وجود قصد متن‌باز بودن، هنوز کامیت نشده است |

### مواردی که عمداً تصمیم‌گیری نشده‌اند

برای این‌ها پاسخی فرض نکنید؛ از روی انتخاب باز مانده‌اند:

- **مونولیت یا میکروسرویس.** این یک سرویس واحد قابل استقرار است. از قبل آن را تکه‌تکه نکنید و مرزهای ماژول
  را برای آینده‌ای فرضی بیش‌ازحد مهندسی نکنید
- **SaaS چندمستأجری / داشبورد.** ممکن است، اما تعهدی به آن نیست. `client_id` و `tenant_id` جا را برایش باز
  گذاشته‌اند؛ الان رابط کاربری مستأجر، صورت‌حساب یا فرایند onboarding نسازید
- **یکپارچگی با سیستم‌های موجود دیگر.** تصمیم‌گیری نشده. do0ps باید یک زیرسیستم تمیز و مستقل باقی بماند که از
  طریق MCP استاندارد در دسترس است — بدون اتصال موردیِ دست‌ساز به هیچ سیستم دیگر

---

## <a id="sec-16"></a>۱۶. مشارکت

۱. اول [`../AGENTS.md`](../AGENTS.md) را بخوانید — مرجع معتبر برای هم انسان‌ها و هم عامل‌های کدنویس است و این
   README خلاصهٔ آن است، نه جایگزینش. (فایل `../CLAUDE.md` فقط اشاره‌ای به آن است تا این دو از هم فاصله نگیرند.)

۲. برنچ خود را از **`develop`** بزنید، نه از `master`، و PR را هم به `develop` بدهید.

۳. یک ایشیو با برچسب `status:ready` بردارید و **پیش از نوشتن کد** آن را به `status:in-progress`
   تغییر دهید.

۴. جهت وابستگی را دست‌نخورده نگه دارید، `go vet` و golangci-lint را تمیز نگه دارید و کنار هر تغییر تست اضافه
   کنید — همهٔ پکیج‌های اینجا تست دارند.

۵. وقتی PR مرج شد و معیارهای پذیرش برآورده شد، ایشیو را ببندید. اگر PR ایشیو را کامل نمی‌بندد،
   `status:in-progress` را نگه دارید و در کامنت بنویسید چه مانده — صف برداشت را وسط کار بی‌صدا به روی بقیه باز
   نکنید.

هنوز فایل لایسنسی کامیت نشده است؛ قصد پروژه متن‌باز بودن است.

</div>
