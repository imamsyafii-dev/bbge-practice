# Fase 02 — Identity and Authentication

Fase ini melanjutkan fondasi Step 1–35 dengan membangun alur identity dan authentication secara bertahap. Urutan belajar yang digunakan adalah **Step 36–65**.

Nomor dan kelompok di bawah ini menjadi urutan belajar yang tetap. Detail implementasi tiap step masih dapat disempurnakan setelah membandingkannya dengan source `bbge-biz` yang aktif pada saat step tersebut dimulai. Dengan cara ini, latihan tetap terarah tanpa menyalin desain lama yang sudah tidak digunakan.

## Prinsip contract practice

- Registrasi practice tidak menerima `email`.
- Client tidak menentukan `user_id`; identity pengguna dibuat oleh server.
- Nama/profil bukan credential dan tidak dijadikan identitas login.
- Password diolah menjadi credential terpisah dan di-hash sebelum data registrasi disimpan.
- Device selalu memiliki ownership yang jelas terhadap account. Mengikuti source aktif, record device sekaligus menyimpan state session seperti refresh token, status, waktu penerbitan, dan waktu kedaluwarsa; practice tidak membuat tabel session umum yang terpisah.
- Pembuatan account, credential, dan device-as-session harus atomic dalam satu transaction.
- JWT practice tidak membawa email.
- Authentication middleware menghasilkan principal yang eksplisit untuk layer berikutnya.

Source production masih memiliki residue legacy berupa field `name` dan `email` pada protobuf serta kolom email pada database/auth storage. Alur registrasi aktif tidak mempercayai `name` atau `email` dari request: server membuat identity internalnya sendiri. Practice sengaja tidak membawa residue email tersebut ke contract baru.

Endpoint password production yang aktif memakai wrapper lebih lengkap: `username`, `password`, nested registration, encrypted identity backup, dan `request_id` untuk retry. Retry dengan `request_id` serta identifier identity yang cocok dapat melanjutkan account yang sudah ada tanpa membuat account kedua; nilai yang tidak cocok menjadi conflict. Alur aktif tidak membandingkan seluruh field request untuk menentukan kecocokan tersebut. Fase ini mempelajari username, hashing, dan registration request ID secara bertahap. Encrypted identity backup serta proof-of-possession/signature verification sengaja berada **di luar Fase 02**; keduanya tidak boleh dianggap terimplementasi oleh latihan sederhana ini.

## Kelompok A — Contract registrasi dan credential (Step 36–42)

### Step 36 — Contract registrasi identity internal

Membuat contract practice sederhana yang diekstrak dari shared inner registration validation: `RegisterInput`, sentinel error, validasi password, device ID, serta panjang public key X25519 dan Ed25519. Ini bukan contract lengkap endpoint password production.

### Step 37 — Password registration wrapper dan policy

Memindahkan password dari inner `RegisterInput` ke outer `PasswordRegisterInput`, lalu memperkenalkan `username`, nested registration, `request_id`, normalisasi username, dan password policy pada contract practice. Step ini baru memvalidasi bahan yang kelak dipakai untuk retry idempotent; perilaku retry terhadap data tersimpan baru diterapkan setelah persistence dan transaction tersedia. Encrypted identity backup tetap di luar fase ini.

### Step 38 — Password credential dan hasher boundary

Membuat bentuk credential terpisah dari account, tipe `EncodedPasswordHash` agar hash tidak diperlakukan sebagai `string` biasa, serta interface consumer `PasswordHasher` yang hanya meminta operasi `Hash`. Plaintext hanya hidup pada boundary proses registrasi dan tidak menjadi field account atau credential yang dipersist. Operasi `Verify` boleh tetap menjadi method concrete hasher pada Step 39; interface verifier baru ditambahkan ketika consumer login benar-benar membutuhkannya, bukan dimasukkan lebih awal ke interface registrasi.

### Step 39 — Implementasi dan test password hashing

Menggunakan algoritma password hashing yang sesuai, memverifikasi password benar/salah, dan memastikan hasil hash tidak sama dengan plaintext. Parameter algoritma akan dikunci setelah kembali memeriksa dependency dan kebijakan source aktif.

### Step 40 — Server-generated user identity

Membuat generator `user_id` melalui dependency yang dapat di-fake. Test memastikan input registrasi tidak dapat memilih atau menimpa identity pengguna.

### Step 41 — Device-as-session ownership

Mendefinisikan entity device beserta invariannya: device ID wajib, device dimiliki account, dan record device membawa refresh token, session status, issued-at, serta expired-at. Tidak dibuat entity atau tabel session umum yang terpisah.

### Step 42 — Registration domain service

Menggabungkan wrapper registrasi, validasi contract, password credential, identity generator, aturan idempotency, dan device-as-session pada domain service tanpa persistence nyata. Test mengunci urutan serta kegagalan dependency.

## Kelompok B — Persistence dan transaction (Step 43–49)

### Step 43 — Schema identity, credential, dan device-as-session

Menambahkan migration practice untuk data yang benar-benar diperlukan. Credential password dipisahkan dari account; `registration_request_id` mendukung retry idempotent; state session disimpan pada record device sesuai model production aktif.

### Step 44 — SQLC query account dan credential

Menulis query serta generated store untuk menyimpan account dan password credential tanpa email.

### Step 45 — SQLC query device-as-session

Menulis query device dengan ownership account yang jelas, refresh token, session status, issued-at, expired-at, dan constraint terhadap duplikasi yang relevan.

### Step 46 — Integration test persistence identity

Menguji query baru terhadap PostgreSQL practice nyata di dalam transaction yang di-rollback agar test dapat diulang.

### Step 47 — Transaction boundary

Mendefinisikan abstraksi transaction kecil yang membuat seluruh operasi registrasi memakai query handle yang sama.

### Step 48 — Repository registrasi atomic

Menyimpan account, password credential yang sudah di-hash, dan device-as-session dalam satu transaction.

### Step 49 — Rollback dan conflict test

Membuktikan kegagalan di tengah registrasi tidak meninggalkan data parsial, retry dengan registration request ID serta identity identifier yang cocok tidak membuat account kedua, dan mismatch dipetakan menjadi conflict domain yang stabil.

## Kelompok C — Application, HTTP, dan wiring (Step 50–55)

### Step 50 — Registration application use case

Membuat use case sebagai orchestration boundary antara transport dan domain service, dengan dependency interface sesuai kebutuhan use case.

### Step 51 — Test registration use case

Menguji forwarding input/result, propagasi context, pemetaan error, dan memastikan orchestration tidak mengulang aturan domain.

### Step 52 — HTTP registration contract tanpa email

Membuat request/response DTO untuk endpoint practice dengan username dan registration request ID. Request tidak memiliki `email` atau `user_id`; response tidak membocorkan password hash.

### Step 53 — Test registration HTTP handler

Menguji decode, validasi transport, status sukses, serta mapping sentinel error menjadi HTTP response yang konsisten.

### Step 54 — Dependency wiring registrasi

Merangkai config, store, transaction runner, hasher, generator, domain service, use case, dan router pada composition root.

### Security boundary sebelum E2E

Sebelum menjalankan Step 55, ingat bahwa alur practice belum mempunyai proof-of-possession/signature verification dan encrypted identity backup. E2E berikut hanya boleh dipakai untuk membuktikan composition, hashing-before-persistence, transaction, device-as-session, serta idempotency versi latihan. Hasilnya tidak membuktikan keamanan protocol production.

### Step 55 — Registration end-to-end practice

Menjalankan migration dan endpoint terhadap PostgreSQL practice, lalu memverifikasi account, credential, serta device-as-session tersimpan secara atomic dan retry request ID yang sama tidak membuat account kedua. Ini hanya memvalidasi arsitektur practice yang disederhanakan, bukan keamanan yang setara production.

## Kelompok D — Access token, refresh, dan principal (Step 56–62)

### Step 56 — JWT configuration boundary

Menambahkan konfigurasi issuer, audience, lifetime, dan signing secret dengan validasi startup serta prefix environment `PRACTICE_`.

### Step 57 — Access-token claims tanpa email

Mendefinisikan claims minimum: subject/user ID, device ID, issuer, audience, issued-at, dan expiry. Jika `session_id` digunakan, nilainya hanya claim transient untuk korelasi token dan tidak menyiratkan tabel session terpisah. Email tidak dimasukkan.

### Step 58 — Token issuer dan verifier

Membuat signing/verifying service beserta test signature, issuer, audience, expiry, dan malformed token.

### Step 59 — Refresh-token persistence dan rotation

Menyimpan representasi refresh token yang aman pada record device-as-session dan menggantinya saat refresh berhasil.

### Step 60 — Refresh use case dan replay test

Menguji refresh sukses, device session yang expired/revoked, reuse token lama, serta kegagalan transaction agar rotation tidak setengah jadi.

### Step 61 — Principal dan context helper

Membuat tipe principal eksplisit dan helper context typed. Principal berisi identity hasil autentikasi, bukan request login mentah.

### Step 62 — Claims menjadi principal

Memvalidasi claims lalu membentuk principal, termasuk pemeriksaan bahwa subject dan device tersedia serta konsisten. Claim session transient diperiksa bila contract token memang menggunakannya.

## Kelompok E — Authentication middleware dan verifikasi akhir (Step 63–65)

### Step 63 — Authentication middleware

Membaca Bearer token, memverifikasinya, membentuk principal, dan menyimpannya ke request context. Error missing, malformed, invalid, dan expired dibedakan.

### Step 64 — Protected route dan wiring test

Membuat route latihan yang memerlukan principal serta test bahwa request anonymous ditolak dan identity terverifikasi diteruskan ke handler/use case.

### Step 65 — Verifikasi akhir identity and authentication

Menjalankan test, race test, build, migration idempotency, registration E2E, access-token authorization, refresh rotation, restart, dan graceful shutdown. Hasil aktual didokumentasikan sebagai penutup fase.

## Batas keamanan Fase 02

Fase ini **tidak** mengimplementasikan proof-of-possession key, verifikasi signature/signed device payload, atau encrypted identity backup production. Konsekuensinya:

- public key pada latihan hanya diperiksa bentuk/panjangnya;
- latihan belum membuktikan client memiliki private key pasangan;
- latihan tidak menyediakan backup identity terenkripsi untuk pemulihan key;
- hasil E2E Step 55 dan verifikasi Step 65 tidak boleh disebut production-equivalent security.

Fitur tersebut membutuhkan desain protocol, pengelolaan key, threat model, dan interoperabilitas client tersendiri. Ia akan menjadi fase lanjutan terpisah jika dipelajari.

## Dependency chain yang akan terbentuk

```text
HTTP request
    -> registration/auth middleware atau handler
    -> application use case
    -> identity domain service
    -> password hasher / identity generator
    -> transaction boundary
    -> account + credential + device-as-session store
    -> PostgreSQL practice
```

Untuk request terautentikasi:

```text
Authorization: Bearer <access token>
    -> token verifier
    -> validated claims
    -> principal
    -> request context
    -> protected handler/use case
```

Setiap step tetap mengikuti pola latihan: tulis manual, format, jalankan test yang fokus, jalankan seluruh test, periksa hasil, dokumentasikan offline, lalu commit satu step.
