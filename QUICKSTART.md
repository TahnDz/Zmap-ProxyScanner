# Quick Start Guide - Raw Socket Testing

## Scenario của bạn: Test Minecraft Server

Bạn muốn test proxy bằng cách gửi packet `b'\x14\x00\x86\x06\rmc.tahn.io.vnc\xdd\x01'` tới `mc.tahn.site:25565`.

### Bước 1: Tạo hex string

```python
# Python
data = b'\x14\x00\x86\x06\rmc.tahn.io.vnc\xdd\x01'
hex_string = data.hex()
print(hex_string)
# Output: 1400860D6D632E7468616E2E696F2E766E63DD01
```

### Bước 2: Tạo config file

Tạo file `config.json`:

```json
{
  "check-sites": [],
  
  "raw_socket_test": {
    "enabled": true,
    "protocol": "tcp",
    "target_host": "mc.tahn.site",
    "target_port": 25565,
    "send_hex": "1400860D6D632E7468616E2E696F2E766E63DD01",
    "expect_response": true,
    "response_min_size": 10,
    "timeout": 5
  },
  
  "proxy-type": "socks5",
  "http_threads": 500,
  
  "headers": {
    "user-agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
    "accept": "text/html,application/xhtml+xml,application/xml"
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

### Bước 3: Build và chạy

```bash
# Build
go build -o proxy_scanner

# Test với file proxies
./proxy_scanner -in proxies.txt -o valid_proxies.txt

# Test với ZMAP
zmap -p 1080 0.0.0.0/0 | ./proxy_scanner -o valid_proxies.txt
```

## Giải thích config

### `enabled: true`
Kích hoạt raw socket testing. Khi bật, sẽ OVERRIDE HTTP checking.

### `protocol: "tcp"`
Dùng TCP protocol. Có thể dùng `"udp"` nếu cần (chỉ SOCKS5).

### `target_host: "mc.tahn.site"`
Server Minecraft của bạn.

### `target_port: 25565`
Port Minecraft (default: 25565).

### `send_hex: "..."`
Raw bytes sẽ gửi, format HEX.

### `expect_response: true`
**true** = Proxy chỉ hợp lệ nếu server reply.
**false** = Proxy hợp lệ nếu gửi được packet (không cần reply).

### `response_min_size: 10`
Response phải >= 10 bytes. Set = 0 nếu không quan tâm size.

### `timeout: 5`
Timeout 5 giây cho mỗi operation.

## Proxy Types

### SOCKS5 (Recommended)
```json
{"proxy-type": "socks5"}
```
- Hỗ trợ domain name (không cần resolve)
- Hỗ trợ TCP và UDP
- Full featured

### SOCKS4
```json
{"proxy-type": "socks4"}
```
- Chỉ IPv4
- Chỉ TCP
- Tự động resolve domain → IP

### HTTP
```json
{"proxy-type": "http"}
```
- Dùng CONNECT method
- Versatile
- Work với most protocols

## Kết quả

### Proxy hợp lệ (Success)
- Kết nối tới proxy thành công ✓
- Establish tunnel tới mc.tahn.site:25565 ✓
- Gửi hex packet thành công ✓
- Nhận response từ server ✓
- Response size >= 10 bytes ✓

→ **Lưu vào `valid_proxies.txt`**

### Proxy error
- Không kết nối được tới proxy ✗
- Proxy handshake fail ✗
- Không establish được tunnel ✗

→ **ProxyErr counter tăng**

### Timeout
- Connection timeout ✗
- Send timeout ✗
- Response timeout ✗

→ **TimeoutErr counter tăng**

### Status code error
- Proxy reject connection ✗
- Server không reply (expect_response = true) ✗
- Response size < 10 bytes ✗

→ **StatusCodeErr counter tăng**

## Output

### Console
```
Imported [1000] IPs Checked [500] IPs (Success: 50, StatusCodeErr: 100, ProxyErr: 200, Timeout: 150) with 500 open http threads
New Proxy 1.2.3.4:1080 Country: US ISP: CloudFlare
New Proxy 5.6.7.8:8080 Country: SG ISP: Digital Ocean
...
```

### valid_proxies.txt
```
1.2.3.4:1080
5.6.7.8:8080
9.10.11.12:3128
...
```

## Các scenario khác

### Chỉ test connection (không gửi data)
```json
{
  "send_hex": "",
  "expect_response": false,
  "response_min_size": 0
}
```

### Gửi data nhưng không đợi reply
```json
{
  "send_hex": "DEADBEEF",
  "expect_response": false
}
```

### Đợi reply bất kỳ size nào
```json
{
  "send_hex": "...",
  "expect_response": true,
  "response_min_size": 0
}
```

## Troubleshooting

### Không có proxy nào pass
1. Test thủ công 1 proxy bằng telnet/netcat
2. Verify hex string đúng format
3. Check target server có alive không
4. Thử giảm `response_min_size`
5. Thử tăng `timeout`

### Quá nhiều timeout
1. Tăng `timeout` value
2. Giảm `http_threads` (500 → 200)
3. Check network latency

### Hex string error
```python
# Validate hex
import binascii
try:
    binascii.unhexlify("1400860D6D632E7468616E2E696F2E766E63DD01")
    print("Valid!")
except:
    print("Invalid hex!")
```

## Advanced: Kết hợp HTTP + Raw Socket

Nếu muốn test cả HTTP và raw socket, chạy 2 lần:

**Run 1: HTTP checking**
```bash
./proxy_scanner -in proxies.txt -cfg config_http.json -o http_valid.txt
```

**Run 2: Raw socket testing**
```bash
./proxy_scanner -in http_valid.txt -cfg config_minecraft.json -o final_valid.txt
```

Hoặc viết script:
```bash
#!/bin/bash
./proxy_scanner -in all_proxies.txt -cfg config_http.json -o http_ok.txt
./proxy_scanner -in http_ok.txt -cfg config_minecraft.json -o minecraft_ok.txt
echo "Final valid proxies in minecraft_ok.txt"
```
