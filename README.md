# ZmapProxyScanner - Phiên bản nâng cấp

## Các cải tiến chính

### 1. Hỗ trợ nhiều check-sites với status code tùy chỉnh
Bây giờ bạn có thể cấu hình nhiều trang web để kiểm tra proxy, mỗi trang có thể có:
- URL riêng
- Status code mong đợi riêng (không chỉ 200)
- Kiểm tra nội dung response (optional)

### 2. Kiểm tra response content
Bạn có thể kiểm tra xem response có chứa các chuỗi cụ thể hay không.

### 3. **RAW SOCKET TESTING** ⭐ NEW
Tính năng hoàn toàn mới cho phép test proxy bằng cách:
- Kết nối tới bất kỳ IP:Port nào
- Gửi raw bytes (hex format) tùy chỉnh
- Hỗ trợ TCP và UDP protocols
- Kiểm tra response từ server
- Hoàn hảo cho: Minecraft servers, game servers, custom protocols

**Ví dụ:** Test proxy với Minecraft server
```json
{
  "raw_socket_test": {
    "enabled": true,
    "protocol": "tcp",
    "target_host": "mc.tahn.site",
    "target_port": 25565,
    "send_hex": "1400860D6D632E7468616E2E696F2E766E63DD01",
    "expect_response": true,
    "response_min_size": 10
  }
}
```

Xem chi tiết trong [README.RAW_SOCKET.md](README.RAW_SOCKET.md)

### 4. SOCKS4/SOCKS5 kiểm tra trực tiếp
- Kết nối trực tiếp tới proxy SOCKS4/5
- Gửi packet handshake chuẩn
- Nhận và kiểm tra phản hồi
- Không còn phụ thuộc vào thư viện bên ngoài

### 5. Sửa lỗi thread leak
- Thêm mutex `processingMu` để tránh xử lý trùng lặp
- Map `processing` để track các proxy đang được xử lý
- Cleanup đúng cách trong defer
- Tránh việc cùng 1 proxy được kiểm tra nhiều lần đồng thời

## Cấu hình mới (config.json)

```json
{
  "check-sites": [
    {
      "url": "https://google.com",
      "status_code": 200,
      "response_contains": []
    },
    {
      "url": "https://api.example.com/status",
      "status_code": 201,
      "response_contains": ["success", "active"]
    }
  ],
  "proxy-type": "http",
  "http_threads": 500,
  "headers": {
    "user-agent": "Mozilla/5.0 ...",
    "accept": "text/html,..."
  },
  "print_ips": {
    "enabled": true,
    "display-ip-info": true
  },
  "timeout": {
    "http_timeout": 5,
    "socks4_timeout": 5,
    "socks5_timeout": 5
  }
}
```

## Giải thích các trường mới

### check-sites (array)
Danh sách các trang web để kiểm tra. Proxy chỉ được coi là hợp lệ nếu PASS TẤT CẢ các site.

- **url**: URL cần kiểm tra
- **status_code**: Mã HTTP status mong đợi (mặc định: 200)
- **response_contains**: Array các chuỗi phải có trong response body (optional, để rỗng [] nếu không cần)

### Ví dụ cấu hình

#### Kiểm tra đơn giản - chỉ status code
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

#### Kiểm tra nâng cao - với response validation
```json
{
  "check-sites": [
    {
      "url": "https://google.com",
      "status_code": 200,
      "response_contains": ["Google"]
    },
    {
      "url": "https://api.ipify.org",
      "status_code": 200,
      "response_contains": []
    }
  ]
}
```

#### Kiểm tra status code khác 200
```json
{
  "check-sites": [
    {
      "url": "https://httpstat.us/201",
      "status_code": 201,
      "response_contains": []
    }
  ]
}
```

## Cải tiến SOCKS4/SOCKS5

### SOCKS4
- Kết nối TCP trực tiếp tới proxy
- Gửi SOCKS4 CONNECT request (format chuẩn)
- Test với 8.8.8.8:80
- Kiểm tra response code (0x5A = success)

### SOCKS5
- Kết nối TCP trực tiếp
- Handshake: no authentication (method 0x00)
- Gửi CONNECT request
- Kiểm tra response code (0x00 = success)

## Sử dụng

```bash
# Build
go build -o proxy_scanner

# Chạy với file input
./proxy_scanner -in proxies.txt -o valid_proxies.txt

# Chạy với API
./proxy_scanner -url https://api.example.com/proxies -o valid_proxies.txt

# Chạy với ZMAP (stdin)
zmap -p 8080 0.0.0.0/0 | ./proxy_scanner -o valid_proxies.txt

# Custom config
./proxy_scanner -in proxies.txt -cfg my_config.json
```

## Ví dụ proxy type

### HTTP Proxy
```json
{
  "proxy-type": "http",
  ...
}
```

### SOCKS4 Proxy
```json
{
  "proxy-type": "socks4",
  ...
}
```

### SOCKS5 Proxy
```json
{
  "proxy-type": "socks5",
  ...
}
```

## Performance

- Giảm số thread mặc định từ 2000 xuống 500 để ổn định hơn
- Thêm sleep 50ms trong worker loop để tránh CPU spike
- Cleanup threads đúng cách để tránh leak
- Processing map để tránh duplicate checking

## Lưu ý

1. Tất cả các check-sites phải PASS thì proxy mới được coi là hợp lệ
2. Nếu `response_contains` có giá trị, ALL chuỗi phải có trong response
3. SOCKS4/5 giờ test bằng cách kết nối thật, không qua HTTP wrapper
4. Thread count nên giữ ở mức 200-1000 tùy theo server performance

## Các file đã thay đổi

- `main.go`: Thêm struct CheckSite, validate config
- `http.go`: Logic kiểm tra nhiều sites, SOCKS handshake mới, fix thread leak
- `config.json`: Cấu trúc mới với check-sites array
