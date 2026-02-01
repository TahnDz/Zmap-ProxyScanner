# CHANGELOG

## Version 2.0 - Major Update

### ✨ Tính năng mới

#### 1. Multi-Site Checking
- Hỗ trợ kiểm tra proxy với nhiều trang web cùng lúc
- Proxy chỉ hợp lệ khi PASS tất cả các site
- Cấu hình linh hoạt cho từng site

#### 2. Custom Status Code
- Không còn bị giới hạn ở status code 200
- Mỗi site có thể có status code riêng (200, 201, 202, 301, v.v.)
- Hữu ích khi test với các API hoặc redirect endpoints

#### 3. Response Content Validation
- Kiểm tra xem response có chứa các chuỗi cụ thể
- Hỗ trợ nhiều chuỗi cho mỗi site
- Tất cả chuỗi phải có trong response thì mới PASS

#### 4. SOCKS4/SOCKS5 Direct Connection
- **SOCKS4**: Gửi trực tiếp SOCKS4 CONNECT packet
  - Format: VER(1) CMD(1) PORT(2) IP(4) USERID(0) NULL(1)
  - Test với 8.8.8.8:80
  - Kiểm tra response: 0x5A = granted
  
- **SOCKS5**: Handshake chuẩn SOCKS5
  - Authentication: No auth (method 0x00)
  - CONNECT request với IPv4
  - Test với 8.8.8.8:80
  - Kiểm tra response: 0x00 = success

### 🐛 Bug Fixes

#### Thread Leak Fix
**Vấn đề**: Script tạo ra quá nhiều threads và không cleanup đúng cách, dẫn đến:
- Memory leak
- CPU spike
- Duplicate checking của cùng 1 proxy

**Giải pháp**:
1. Thêm `processingMu` mutex để quản lý processing state
2. Thêm map `processing` để track proxies đang được kiểm tra
3. Cleanup trong defer function:
   ```go
   defer func() {
       atomic.AddInt64(&p.openHttpThreads, -1)
       atomic.AddUint64(&checked, 1)
       p.processingMu.Lock()
       delete(p.processing, proxy)
       p.processingMu.Unlock()
   }()
   ```
4. Kiểm tra proxy đã được processing chưa trước khi tạo goroutine mới
5. Thêm sleep 50ms trong worker loop để giảm CPU usage

### ⚡ Performance Improvements

1. **Giảm default threads**: 2000 → 500
   - Ổn định hơn
   - Ít resource intensive hơn
   - Vẫn đủ nhanh cho hầu hết use cases

2. **Better resource management**
   - Cleanup connections đúng cách
   - Timeout cho tất cả network operations
   - Defer close cho connections

3. **Sleep optimization**
   - 100ms → 50ms trong worker loop
   - Giảm CPU usage mà không ảnh hưởng performance đáng kể

### 📝 Configuration Changes

#### Cũ (config v1):
```json
{
  "check-site": "https://google.com",
  ...
}
```

#### Mới (config v2):
```json
{
  "check-sites": [
    {
      "url": "https://google.com",
      "status_code": 200,
      "response_contains": []
    }
  ],
  ...
}
```

### 🔄 Breaking Changes

1. **Config format**: `check-site` (string) → `check-sites` (array)
2. **SOCKS implementation**: Từ wrapper library → Direct TCP connection
3. **Thread count default**: 2000 → 500

### 📋 Migration Guide

#### Updating config.json

**Before**:
```json
{
  "check-site": "https://google.com"
}
```

**After**:
```json
{
  "check-sites": [
    {
      "url": "https://google.com",
      "status_code": 200,
      "response_contains": []
    }
  ]
}
```

### 🎯 Use Cases

#### Case 1: Simple HTTP Proxy Check
```json
{
  "check-sites": [
    {"url": "https://google.com", "status_code": 200, "response_contains": []}
  ],
  "proxy-type": "http"
}
```

#### Case 2: API Endpoint with Custom Status
```json
{
  "check-sites": [
    {"url": "https://api.example.com/create", "status_code": 201, "response_contains": ["success"]}
  ],
  "proxy-type": "http"
}
```

#### Case 3: Multiple Site Validation
```json
{
  "check-sites": [
    {"url": "https://google.com", "status_code": 200, "response_contains": ["Google"]},
    {"url": "https://api.ipify.org", "status_code": 200, "response_contains": []}
  ],
  "proxy-type": "http"
}
```

#### Case 4: SOCKS4 Direct Test
```json
{
  "check-sites": [
    {"url": "https://google.com", "status_code": 200, "response_contains": []}
  ],
  "proxy-type": "socks4"
}
```

### 📊 Technical Details

#### Thread Management
```
Old: Create goroutine → No tracking → Possible duplicates → Leak
New: Check processing → Lock → Create goroutine → Track → Cleanup
```

#### SOCKS4 Handshake
```
Client → Server: [0x04][0x01][PORT][IP][0x00]
Server → Client: [0x00][STATUS][PORT][IP]
STATUS: 0x5A (granted) or 0x5B (rejected)
```

#### SOCKS5 Handshake
```
1. Client → Server: [0x05][0x01][0x00] (version, nmethods, no auth)
2. Server → Client: [0x05][0x00] (version, method)
3. Client → Server: [0x05][0x01][0x00][0x01][IP][PORT] (CONNECT)
4. Server → Client: [0x05][STATUS][...] (STATUS: 0x00 = success)
```

### 🔍 Testing

Recommended testing flow:
1. Test với 1 site đơn giản trước
2. Test với nhiều sites
3. Test với response validation
4. Test với custom status codes
5. Test SOCKS4/5 nếu cần

### ⚠️ Known Limitations

1. Response validation chỉ kiểm tra text trong body, không hỗ trợ regex
2. SOCKS authentication chưa được implement (chỉ hỗ trợ no-auth)
3. IPv6 chưa được hỗ trợ trong SOCKS handshake

### 🚀 Future Improvements

- [ ] Hỗ trợ regex trong response validation
- [ ] SOCKS authentication (username/password)
- [ ] IPv6 support
- [ ] Metrics export (Prometheus format)
- [ ] Web dashboard
- [ ] Database storage option
