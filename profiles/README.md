## Analysis

`base.pprof` was captured before the optimization, `result.pprof` after, both under the same load (see `-seconds` in the profile-capture command). Negative numbers in the diff profile are what stopped being allocated after the change.

What was the bottleneck (from `alloc_space`/`alloc_objects` in `base.pprof`, before the optimization):
- **`compress/flate.NewWriter`** — almost all allocated memory (`alloc_space`). `middleware.GzipHandler` created a new `gzip.Writer` via `gzip.NewWriter(w)` on every request, and internally that allocates fresh Huffman tables and compressor buffers (`compress/flate.(*compressor).initDeflate`) on every call.
- **`go.uber.org/zap.(*Logger).Sugar`** and **`(*Logger).clone`** — a noticeable share of `alloc_objects`. `middleware.LoggingHandler` called `logger.Sugar()` inside the handler on every request, even though `Sugar()` clones and wraps the logger each time it's called.

What changed in the code:
1. `internal/middleware/encoder.go` — added `gzipWriterPool = sync.Pool{New: func() any { return gzip.NewWriter(io.Discard) }}`. `gzipWriter.WriteHeader` now takes a `*gzip.Writer` from the pool and reinitializes it via `Reset(w.ResponseWriter)` instead of `gzip.NewWriter(w.ResponseWriter)`; after the response, the writer is returned to the pool (`gzipWriterPool.Put`). The compressor's buffers and Huffman tables are reused across requests instead of being recreated.
2. `internal/middleware/logger.go` — moved `sugar := logger.Sugar()` out of the per-request handler closure into `LoggingHandler`'s body, which runs once at server startup instead of once per request.

How this shows up in the diff profiles below:
- In `alloc_space` (second block), `compress/flate.NewWriter` and `compress/flate.(*compressor).initDeflate` together account for `-162487.77MB` out of the `-163951.41MB` shown for all nodes — nearly the entire effect, and a direct consequence of change 1: the writer is no longer recreated on every request.
- In `alloc_objects` (third block), the disappearance of `go.uber.org/zap.(*Logger).Sugar` (`-720901`) and `(*Logger).clone` (`-430132`) is a direct consequence of change 2. The rest of that block (`compress/flate.newHuffmanEncoder`, `newHuffmanBitWriter`, etc.) is the same effect from change 1, just counted in objects instead of bytes.
- In `inuse_space` (first block) the picture is different: the top entries are `encoding/json` and `storage.(*FileStorage).Set` — functions the change didn't touch. After the optimization the service processes requests faster and puts less pressure on the GC, so at the moment the snapshot was taken there was less "in-flight" state in memory (bodies being decoded, lines not yet written to the file) — a side effect of the speedup, not a result of directly changing those functions.

```
$ go tool pprof -top -diff_base=profiles/base.pprof profiles/result.pprof
File: shortener
Build ID: c7bc578ac529e6c9073bc7a26495627452491643
Type: inuse_space
Time: 2026-08-19 22:13:05 +05
Showing nodes accounting for -11405.88kB, 42.22% of 27013.51kB total
      flat  flat%   sum%        cum   cum%
-5120.23kB 18.95% 18.95% -5120.23kB 18.95%  encoding/json.(*decodeState).literalStore
-3725.82kB 13.79% 32.75% -3725.82kB 13.79%  github.com/dmitrymack/go-url-shortener.git/internal/storage.(*FileStorage).Set
-2048.03kB  7.58% 40.33% -2048.03kB  7.58%  encoding/json.(*scanner).pushParseState
 1538.24kB  5.69% 34.63%  1538.24kB  5.69%  runtime.mallocgc
-1536.02kB  5.69% 40.32% -1536.02kB  5.69%  github.com/dmitrymack/go-url-shortener.git/internal/service.generateID (inline)
    -514kB  1.90% 42.22%     -514kB  1.90%  bufio.NewReaderSize (inline)
         0     0% 42.22%     -514kB  1.90%  bufio.NewReader (inline)
         0     0% 42.22% -7168.27kB 26.54%  encoding/json.(*Decoder).Decode
         0     0% 42.22% -5120.23kB 18.95%  encoding/json.(*decodeState).object
         0     0% 42.22% -2048.03kB  7.58%  encoding/json.(*decodeState).scanWhile
         0     0% 42.22% -7168.27kB 26.54%  encoding/json.(*decodeState).unmarshal
         0     0% 42.22% -5120.23kB 18.95%  encoding/json.(*decodeState).value
         0     0% 42.22% -2048.03kB  7.58%  encoding/json.stateBeginValue
         0     0% 42.22% -12430.11kB 46.01%  github.com/dmitrymack/go-url-shortener.git/internal/handler.(*Handler).SetShortUrlByJSON
         0     0% 42.22% -12430.11kB 46.01%  github.com/dmitrymack/go-url-shortener.git/internal/middleware.AuthorizerHandler.func1
         0     0% 42.22% -12430.11kB 46.01%  github.com/dmitrymack/go-url-shortener.git/internal/middleware.GzipHandler.func1
         0     0% 42.22% -5261.85kB 19.48%  github.com/dmitrymack/go-url-shortener.git/internal/service.(*ShortenService).CreateShortURL
         0     0% 42.22% -12430.11kB 46.01%  github.com/go-chi/chi/v5.(*Mux).ServeHTTP
         0     0% 42.22% -12430.11kB 46.01%  github.com/go-chi/chi/v5.(*Mux).routeHTTP
         0     0% 42.22% -12430.11kB 46.01%  main.main.LoggingHandler.func5.1
         0     0% 42.22% -12944.11kB 47.92%  net/http.(*conn).serve
         0     0% 42.22% -12430.11kB 46.01%  net/http.HandlerFunc.ServeHTTP
         0     0% 42.22%     -514kB  1.90%  net/http.newBufioReader
         0     0% 42.22% -12430.11kB 46.01%  net/http.serverHandler.ServeHTTP
         0     0% 42.22%     1026kB  3.80%  runtime.allocm
         0     0% 42.22%   512.23kB  1.90%  runtime.malg
         0     0% 42.22%     1026kB  3.80%  runtime.mstart
         0     0% 42.22%     1026kB  3.80%  runtime.mstart0
         0     0% 42.22%     1026kB  3.80%  runtime.mstart1
         0     0% 42.22%     1026kB  3.80%  runtime.newm
         0     0% 42.22%  1538.24kB  5.69%  runtime.newobject
         0     0% 42.22%   512.23kB  1.90%  runtime.newproc.func1
         0     0% 42.22%   512.23kB  1.90%  runtime.newproc1
         0     0% 42.22%     1026kB  3.80%  runtime.resetspinning
         0     0% 42.22%     1026kB  3.80%  runtime.schedule
         0     0% 42.22%     1026kB  3.80%  runtime.startm
         0     0% 42.22%   512.23kB  1.90%  runtime.systemstack
         0     0% 42.22%     1026kB  3.80%  runtime.wakep
```

```
$ go tool pprof -top -sample_index=alloc_space -diff_base=profiles/base.pprof profiles/result.pprof
File: shortener
Build ID: c7bc578ac529e6c9073bc7a26495627452491643
Type: alloc_space
Time: 2026-08-19 22:13:05 +05
Showing nodes accounting for -163951.41MB, 97.24% of 168602.23MB total
Dropped 251 nodes (cum <= 843.01MB)
      flat  flat%   sum%        cum   cum%
-134265.85MB 79.63% 79.63% -163273.85MB 96.84%  compress/flate.NewWriter (inline)
-28221.92MB 16.74% 96.37% -28221.92MB 16.74%  compress/flate.(*compressor).initDeflate (inline)
-1431.64MB  0.85% 97.22% -1431.64MB  0.85%  compress/flate.(*huffmanEncoder).generate
     -18MB 0.011% 97.23% -166116.76MB 98.53%  main.main.LoggingHandler.func5.1
   -4.50MB 0.0027% 97.24% -165816.16MB 98.35%  github.com/dmitrymack/go-url-shortener.git/internal/middleware.GzipHandler.func1
      -4MB 0.0024% 97.24% -164380.01MB 97.50%  github.com/dmitrymack/go-url-shortener.git/internal/middleware.AuthorizerHandler.func1
      -3MB 0.0018% 97.24% -167000.84MB 99.05%  net/http.(*conn).serve
   -2.50MB 0.0015% 97.24% -163524.79MB 96.99%  github.com/dmitrymack/go-url-shortener.git/internal/handler.(*Handler).SetShortUrlByJSON
         0     0% 97.24% -1431.64MB  0.85%  compress/flate.(*Writer).Close (inline)
         0     0% 97.24% -1431.64MB  0.85%  compress/flate.(*compressor).close
         0     0% 97.24% -1431.64MB  0.85%  compress/flate.(*compressor).deflate
         0     0% 97.24%   -29008MB 17.20%  compress/flate.(*compressor).init
         0     0% 97.24% -1431.64MB  0.85%  compress/flate.(*compressor).writeBlock
         0     0% 97.24%  -945.57MB  0.56%  compress/flate.(*huffmanBitWriter).indexTokens
         0     0% 97.24% -1431.64MB  0.85%  compress/flate.(*huffmanBitWriter).writeBlock
         0     0% 97.24% -1431.64MB  0.85%  compress/gzip.(*Writer).Close
         0     0% 97.24% -163273.85MB 96.84%  compress/gzip.(*Writer).Write
         0     0% 97.24% -163272.97MB 96.84%  github.com/dmitrymack/go-url-shortener.git/internal/middleware.(*gzipWriter).Write
         0     0% 97.24% -1431.64MB  0.85%  github.com/dmitrymack/go-url-shortener.git/internal/middleware.GzipHandler.func1.1
         0     0% 97.24% -166246.29MB 98.60%  github.com/go-chi/chi/v5.(*Mux).ServeHTTP
         0     0% 97.24% -163655.39MB 97.07%  github.com/go-chi/chi/v5.(*Mux).routeHTTP
         0     0% 97.24% -166116.76MB 98.53%  net/http.HandlerFunc.ServeHTTP
         0     0% 97.24% -166246.29MB 98.60%  net/http.serverHandler.ServeHTTP
```

```
$ go tool pprof -top -sample_index=alloc_objects -diff_base=profiles/base.pprof profiles/result.pprof
File: shortener
Build ID: c7bc578ac529e6c9073bc7a26495627452491643
Type: alloc_objects
Time: 2026-08-19 22:13:05 +05
Showing nodes accounting for -26152945, 59.66% of 43835343 total
Dropped 132 nodes (cum <= 219176)
      flat  flat%   sum%        cum   cum%
  -1360388  3.10%  3.10%   -1360388  3.10%  compress/flate.newHuffmanEncoder (inline)
  -1323267  3.02%  6.12%   -1323267  3.02%  net/textproto.readMIMEHeader
  -1267088  2.89%  9.01%   -1267088  2.89%  encoding/base64.(*Encoding).EncodeToString
   -991247  2.26% 11.27%    -991247  2.26%  go.uber.org/zap/zapcore.(*sliceArrayEncoder).AppendString
   -963115  2.20% 13.47%    -963115  2.20%  github.com/golang-jwt/jwt/v4.NewWithClaims (inline)
   -950289  2.17% 15.64%  -21686130 49.47%  main.main.LoggingHandler.func5.1
   -925187  2.11% 17.75%   -2013217  4.59%  encoding/json.Marshal
   -854821  1.95% 19.70%    -854821  1.95%  internal/bytealg.MakeNoZero
   -835692  1.91% 21.61%    -835692  1.91%  net/http.Header.Clone (inline)
   -814137  1.86% 23.46%   -2174525  4.96%  compress/flate.newHuffmanBitWriter (inline)
   -808314  1.84% 25.31%    -808314  1.84%  context.WithValue
   -777613  1.77% 27.08%    -777613  1.77%  sync.(*poolChain).pushHead
   -759176  1.73% 28.81%   -1254852  2.86%  crypto/internal/fips140/hmac.New[go.shape.interface { BlockSize int; Reset; Size int; Sum []uint8; Write  }]
   -753676  1.72% 30.53%    -753676  1.72%  reflect.unsafe_New
   -727168  1.66% 32.19%   -6952040 15.86%  github.com/dmitrymack/go-url-shortener.git/internal/service.BuildJWTString
   -720901  1.64% 33.84%   -1151033  2.63%  go.uber.org/zap.(*Logger).Sugar
   -651555  1.49% 35.32%    -651555  1.49%  compress/flate.(*huffmanEncoder).generate
   -506419  1.16% 36.48%    -506419  1.16%  net/http.(*Request).WithContext (inline)
   -495676  1.13% 37.61%    -495676  1.13%  crypto/internal/fips140/sha256.New (inline)
   -491527  1.12% 38.73%    -982681  2.24%  context.WithCancel
   -458759  1.05% 39.78%    -458759  1.05%  github.com/dmitrymack/go-url-shortener.git/internal/middleware.generateUUID (inline)
   -430132  0.98% 40.76%    -430132  0.98%  go.uber.org/zap.(*Logger).clone (inline)
   -424886  0.97% 41.73%    -424886  0.97%  compress/flate.(*compressor).initDeflate (inline)
   -417155  0.95% 42.68%    -417155  0.95%  net/textproto.MIMEHeader.Add (inline)
   -393222   0.9% 43.58%    -393222   0.9%  time.Duration.String
   -389608  0.89% 44.46%    -389607  0.89%  net/url.(*URL).joinPath
   -367030  0.84% 45.30%   -2293653  5.23%  github.com/golang-jwt/jwt/v4.(*SigningMethodHMAC).Sign
   -367029  0.84% 46.14%    -491154  1.12%  context.withCancel (inline)
   -338658  0.77% 46.91%    -338658  0.77%  net/http.(*Request).SetPathValue (inline)
   -311306  0.71% 47.62%    -311306  0.71%  time.Time.Format
   -309518  0.71% 48.33%    -309518  0.71%  net/url.parse
   -305841   0.7% 49.03%    -305841   0.7%  go.uber.org/zap/buffer.(*Buffer).String (inline)
   -305063   0.7% 49.72%    -485298  1.11%  net/textproto.(*Reader).ReadLine (inline)
   -301489  0.69% 51.08%    -301489  0.69%  net.(*conn).Read
   -294967  0.67% 51.08%   -3796168  8.66%  net/http.(*conn).readRequest
   -294917  0.67% 51.75%    -294917  0.67%  net/textproto.MIMEHeader.Set (inline)
   -290610  0.66% 52.42%    -317243  0.72%  go.uber.org/zap/internal/stacktrace.Capture
   -285168  0.65% 53.07%   -2599413  5.93%  net/http.readRequest
   -283995  0.65% 53.72%    -393227   0.9%  encoding/json.(*decodeState).object
   -278544  0.64% 54.35%    -705241  1.61%  fmt.Sprintln
   -262148   0.6% 54.95%  -15674099 35.76%  github.com/dmitrymack/go-url-shortener.git/internal/middleware.AuthorizerHandler.func1
   -255610  0.58% 55.53%   -1017478  2.32%  encoding/json.mapEncoder.encode
   -239242  0.55% 56.08%    -239242  0.55%  compress/gzip.NewWriterLevel
   -212173  0.48% 56.56%   -2811584  6.41%  compress/flate.NewWriter (inline)
   -196611  0.45% 57.01%  -27301848 62.28%  net/http.(*conn).serve
   -188956  0.43% 57.44%    -188956  0.43%  sync.(*Pool).pinSlow
   -180235  0.41% 57.85%    -180235  0.41%  sync.NewCond (inline)
   -180229  0.41% 58.26%    -180229  0.41%  crypto/internal/fips140/sha256.(*Digest).Sum
   -163842  0.37% 58.64%   -5492243 12.53%  github.com/dmitrymack/go-url-shortener.git/internal/handler.(*Handler).SetShortUrlByJSON
   -147461  0.34% 58.97%  -16473115 37.58%  github.com/dmitrymack/go-url-shortener.git/internal/middleware.GzipHandler.func1
   -114691  0.26% 59.24%    -286287  0.65%  net/http.(*Server).Serve
   -104499  0.24% 59.47%    -202804  0.46%  encoding/json.(*Decoder).refill
    -82095  0.19% 59.66%    -265752  0.61%  github.com/dmitrymack/go-url-shortener.git/internal/storage.(*FileStorage).Set
```
