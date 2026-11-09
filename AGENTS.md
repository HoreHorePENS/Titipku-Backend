# AGENTS.md — Development Guidelines & Architectural Standards

Dokumen ini adalah panduan kerja (operating manual) resmi bagi AI Agent dan developer dalam mengembangkan, memelihara, dan merefaktor repositori **Titipku**.

Setiap agen AI dan pengembang yang bekerja di repositori ini **wajib** mematuhi prinsip **Clean Code**, **Design Patterns**, dan arsitektur yang telah distandarkan di bawah ini secara konsisten tanpa kompromi.

---

## 1. Domain & Visi Proyek

**Titipku** adalah platform marketplace konsinyasi UMKM (titip-jual) yang menghubungkan dua aktor utama:
1. **Penitip (Produsen / Consignor):** Produsen kecil/rumahan yang menitipkan produk dagangan mereka ke jaringan warung/toko mitra.
2. **Pemilik Warung/Toko (Store Owner):** Pemilik toko fisik yang menerima titipan barang untuk dijual kepada konsumen akhir.

### Ubiquitous Language (Domain Glossary)
- **Penitip:** Produsen/pemilik barang konsinyasi (`account_type = 'penitip'`).
- **Pemilik Toko / Mitra Warung:** Pemilik fisik gerai/toko (`account_type = 'pemilik_toko'`).
- **Kunjungan (Visit):** Siklus lapangan saat Penitip mendatangi Toko untuk:
  - Menghitung sisa stok fisik (*stock opname*).
  - Mencatat retur (barang rusak/kadaluarsa).
  - Menghitung jumlah barang terjual = `(stok_awal - sisa_stok - retur)`.
  - Menitipkan stok baru (*drop off*).
  - Menerbitkan nota tagihan (invoice) otomatis.
- **Tagihan (Invoice):** Piutang yang wajib dibayarkan oleh Pemilik Toko ke Penitip atas barang yang telah laku terjual.
- **Peta Toko (Store Map):** Fitur geolokasi interaktif berbasis Leaflet untuk menemukan dan mendaftarkan titipan ke warung potensial di sekitar.

---

## 2. Nilai Inti: Clean Code & Standar Desain

### A. Prinsip SOLID
1. **Single Responsibility Principle (SRP):**
   - Satu modul, fungsi, class, atau komponen hanya boleh memiliki satu alasan untuk berubah.
   - *Backend:* Pisahkan HTTP handler (transport), Use Case (orkestrasi bisnis), dan Repository (persistensi DB).
   - *Frontend:* Pisahkan presentasi UI (`components`), pemanggilan data / state (`hooks`), dan validasi/tipe (`schema`/`types`).
2. **Open/Closed Principle (OCP):**
   - Kode harus terbuka untuk ekstensi, namun tertutup untuk modifikasi.
   - Gunakan interface/kontrak. Contoh: Penambahan metode pembayaran baru tidak boleh merusak use case pembayaran yang sudah ada.
3. **Liskov Substitution Principle (LSP):**
   - Implementasi adapter harus dapat menggantikan interface port tanpa merusak ekspektasi use case.
4. **Interface Segregation Principle (ISP):**
   - Buat interface yang ramping dan spesifik. Jangan memaksa konsumen mengimplementasikan method yang tidak mereka butuhkan.
5. **Dependency Inversion Principle (DIP):**
   - Modul level tinggi (Domain/Usecase) tidak boleh bergantung pada modul level rendah (Postgres/HTTP/3rd-party). Keduanya harus bergantung pada abstraksi (Interfaces).

### B. Prinsip Praktis Lainnya
- **KISS (Keep It Simple, Stupid):** Hindari over-engineering. Jangan membuat layer abstraksi jika belum ada kebutuhan variasi nyata.
- **DRY (Don't Repeat Yourself):** Abstraksikan logika bisnis yang berulang, namun bedakan dengan *accidental duplication* (kode mirip tapi konteks domain berbeda).
- **YAGNI (You Aren't Gonna Need It):** Jangan menambahkan fitur spekulatif sebelum ada use case konkret.
- **Boy Scout Rule:** Tinggalkan file dalam keadaan lebih rapi, terstruktur, dan bersih daripada saat pertama kali dibuka.
- **Command-Query Separation (CQS):** Fungsi yang mengubah state (Command) tidak boleh mengembalikan data query kompleks, dan fungsi yang membaca data (Query) tidak boleh menimbulkan efek samping (side effects).

---

## 3. Arsitektur Backend (Go / `services/core-service`)

Backend mengadopsi **Hexagonal Architecture (Ports and Adapters)** dengan pendekatan **Modular Monolith**:

```text
cmd/server/main.go (Wire dependencies)
       │
       ▼
  [Adapter: Inbound] (HTTP Handlers, Middleware, DTOs)
       │
       ▼ calls
  [Usecase] (Business workflows & Orchestration)
       │
       ├── uses ──► [Domain] (Entities, Value Objects, Business Invariants)
       │
       ▼ defines
  [Ports / Interfaces] (Repository Ports, Service Ports)
       ▲
       │ implements
  [Adapter: Outbound] (PostgreSQL Repositories, DB Functions, External APIs)
```

### Aturan Arsitektur Backend
1. **`internal/domain`:**
   - **Murni tanpa dependensi eksternal!** Tidak boleh mengimpor package database (`pgx`, `sql`), framework HTTP, atau usecase.
   - Berisi entity struct, custom error domain (`ErrNotFound`, `ErrUnauthorized`, `ErrValidation`), dan validasi invariant.
2. **`internal/usecase`:**
   - Berisi business logic / workflow.
   - Mendefinisikan port interface yang dibutuhkan (e.g. `PartnerRepository`, `StoreRepository`, `TransactionManager`).
   - Mengembalikan domain error, bukan HTTP status code.
3. **`internal/adapter`:**
   - **HTTP Handler:** Membaca payload, memvalidasi format, memanggil usecase, dan mengembalikan response JSON seragam via envelope `{ "data": ... }` atau `{ "error": { "code": "...", "message": "..." } }`.
   - **Repository:** Mengimplementasikan interface port menggunakan PostgreSQL / `pgx`.
   - Query kompleks multi-tabel untuk visit & invoice dieksekusi di database functions / transaction block untuk menjamin konsistensi ACID.
4. **`cmd/server`:**
   - Melakukan Dependency Injection (DI) secara eksplisit tanpa magic DI framework.

### Design Patterns di Backend
- **Repository Pattern:** Seluruh akses data diabstraksikan melalui interface repository.
- **Factory / Constructor Pattern:** Gunakan constructor `New<Type>()` yang mengembalikan interface atau pointer ke struct tervalidasi.
- **Middleware Pattern:** Logging, CORS, Autentikasi JWT, Rate-Limiting, dan Panic Recovery.
- **Unit of Work / Transaction Pattern:** Transaksi finansial dan perubahan stok harus diikat dalam satu transaksi atomic DB (`BeginTx` / `Commit` / `Rollback`).

---

## 4. Arsitektur Frontend (React 19 / TypeScript / Vite / Tailwind)

Frontend mengadopsi **Feature-Driven Modular Architecture**:

```text
src/
├── app/                  # Application core (Router, Layouts, Providers, Nav)
├── assets/               # Static assets & SVG icons
├── components/           # Shared reusable atomic components
│   ├── shared/           # Generic UI (Button, Modal, Card, Badge, Table, Input)
│   └── feedback/         # Toast, LoadingSpinner, EmptyState
├── features/             # Domain feature modules
│   ├── auth/             # Login, Register, Session management
│   ├── stores/           # Store directory, Map, Consignment request
│   ├── products/         # Product catalog & stock
│   ├── partners/         # Consignment partner/warung management
│   ├── visits/           # Visit flows, stock recording
│   ├── invoices/         # Invoices, payments, receivables
│   └── dashboard/        # Role-based analytics & KPIs
├── lib/                  # Utilities, API client (fetcher/axios), formatting
├── routes/               # Page components corresponding to router URLs
├── styles/               # CSS tokens, theme, Tailwind config
└── types/                # Global cross-cutting TypeScript definitions
```

### Aturan Modular Feature
Setiap direktori di bawah `src/features/<feature-name>/` harus mandiri dan memiliki struktur:
```text
features/<feature>/
├── api.ts            # TanStack Query hooks (queries & mutations)
├── types.ts          # Feature-specific DTOs and view models
├── schema.ts         # Zod schemas for forms and validations
├── components/       # Feature-specific UI components
└── index.ts          # Public barrel export
```

### Design Patterns di Frontend
- **Smart / Dumb Component Pattern (Container / Presenter):**
  - *Dumb:* Komponen presentasi murni menerima props, me-render UI, dan memicu callback event. Bebas dari pemanggilan API langsung.
  - *Smart:* Komponen route/page atau container mengelola data fetching via custom hook dan meneruskan data ke dumb component.
- **Custom Hook Pattern:**
  - Seluruh logic asinkron, pemanggilan API, dan TanStack Query wajib di-encapsulate dalam custom hook (e.g., `useStores()`, `useRecordVisit()`, `useCurrentAccount()`).
- **Compound Component Pattern:**
  - Komponen kompleks seperti `Modal`, `DropdownMenu`, atau `Tabs` menggunakan pola sub-komponen (`Modal.Header`, `Modal.Body`, `Modal.Footer`).
- **Single Source of Truth Validation (Zod):**
  - Tipe form dan validasi runtime diturunkan langsung dari Zod schema (`z.infer<typeof schema>`).

---

## 5. Standar UI/UX: Minimalist Monochrome Design System

Sesuai instruksi desain produk Titipku:
1. **Tema Bersih & Terang (Pure Light Theme):**
   - Latar belakang wajib putih bersih (`#ffffff`) atau abu-abu netral sangat terang (`#f8fafc`, `#f1f5f9`).
   - **TIDAK ADA DARK MODE.** Jangan sertakan utility dark mode atau background hitam pekat untuk keseluruhan halaman.
2. **Tipografi:**
   - Menggunakan font **Plus Jakarta Sans** secara konsisten di seluruh elemen tipografi.
3. **Ikon & Visual:**
   - **DILARANG MENGGUNAKAN EMOJI BERWARNA DI UI.**
   - Gunakan **ikon monokrom SVG murni** (stroke/fill warna hitam, slate, atau abu-abu netral).
   - Indikator status (Lunas, Belum Lunas, Menunggu) menggunakan badge minimalis dengan kontras teks yang jelas dan border halus.
4. **Mobile-First Experience:**
   - Titipku dirancang untuk digunakan di lapangan (mobile view).
   - Ukuran target sentuh (touch target) tombol dan link minimal `44px x 44px`.
   - Navigasi bawah (Bottom Navigation Bar) adaptif sesuai role (`penitip` vs `pemilik_toko`).
   - Top Header dilengkapi tombol logout dan konfirmasi aksi.

---

## 6. Standar Kualitas & Kebersihan Kode (Code Quality)

### Aturan Clean Code Wajib
1. **Penamaan Jelas & Intensional (Intention-Revealing Names):**
   - Hindari singkatan tidak bermakna seperti `usr`, `btn1`, `calc()`, `temp`.
   - Gunakan nama deklaratif: `CalculateConsignmentSoldQuantity()`, `consignToStore()`, `isStoreActive`.
2. **Ukuran Fungsi & Kompleksitas:**
   - Maksimal 30 baris per fungsi kecuali fungsi deklarasi mapping/router.
   - Maksimal 3 argumen per fungsi. Jika butuh lebih dari 3 parameter, bungkus dalam struct/object DTO.
   - Hindari *deep nesting* (`if` di dalam `if` di dalam `for`). Gunakan **Guard Clauses** dan **Early Return Pattern**.
3. **Error Handling Terstruktur:**
   - *Go:* Selalu periksa error secara eksplisit. Bungkus error dengan konteks: `fmt.Errorf("failed to fetch store by id %s: %w", id, err)`. Jangan pernah abaikan `_ = err`.
   - *TypeScript:* Gunakan tipe aman, `try/catch` pada batas asynchronous, dan tangani error state pada TanStack Query secara elegan di UI.
4. **Dead Code & Comments:**
   - Hapus kode yang dikomentari (*commented-out code*) dan impor yang tidak terpakai (*unused imports*). Git adalah penyimpan riwayat kita.
   - Komentar hanya diperbolehkan untuk menjelaskan **"MENGAPA"** (alasan bisnis / keputusan non-trivial), bukan **"APA"** yang dilakukan kode.

---

## 7. Workflow Pengujian (Testing & Verification)

Setiap perubahan kode **wajib** lolos pengujian berikut sebelum dianggap selesai:

### Backend
```powershell
# Jalankan unit & integration tests
go test ./...

# Validasi static analysis & formatting
go vet ./...
gofmt -w .
```

### Frontend
```powershell
# Jalankan unit tests (Vitest)
npm run test

# Jalankan linter
npm run lint

# Pastikan TypeScript compile dan build produksi sukses
npm run build
```

---

## 8. Panduan Aksi AI Agent (Agent Rules of Engagement)

1. **Jaga Integritas Build:** Jangan pernah meninggalkan workspace dalam keadaan tes gagal (`FAIL`) atau build rusak.
2. **Refactor Bertahap:** Saat melakukan perbaikan atau fitur baru, refactor kode yang berdekatan agar semakin bersih (*Boy Scout Rule*), namun jangan mengubah kontrak API publik tanpa persetujuan.
3. **Konsistensi Desain:** Pastikan seluruh komponen UI baru selalu mematuhi tema monokrom, font Plus Jakarta Sans, dan tidak menyertakan ikon emoji berwarna.
4. **Dokumentasi Terkini:** Jika ada endpoint baru atau perubahan skema data, perbarui spesifikasi OpenAPI di `Titipku-Infrastructure/specs/openapi/` dan dokumentasi terkait.
