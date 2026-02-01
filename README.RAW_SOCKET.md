# Raw Socket Testing - Hướng dẫn chi tiết

## Tổng quan

Tính năng Raw Socket Testing cho phép bạn test proxy bằng cách:
1. Kết nối tới một server tùy chỉnh (IP:Port)
2. Gửi raw bytes (hex format) 
3. Chờ response từ server
4. Kiểm tra response có hợp lệ không

Điều này rất hữu ích khi test proxy với các giao thức đặc biệt như:
- Minecraft servers
- Game servers  
- Custom TCP/UDP protocols
- Binary protocols

## Cấu hình

### Cấu trúc JSON

```json
{
  "raw_socket_test": {
    "enabled": true,
    "protocol": "tcp",
    "target_host": "mc.tahn.site",
    "target_port": 25565,
    "send_hex": "1400860D6D632E7468616E2E696F2E766E63DD01",
    "expect_response": true,
    "response_min_size": 10,
    "timeout": 5
  }
}
```

### Các trường cấu hình

#### `enabled` (boolean, bắt buộc)
- `true`: Kích hoạt raw socket testing
- `false`: Tắt raw socket testing, dùng HTTP checking thông thường

#### `protocol` (string, bắt buộc)
Giao thức network cần dùng:
- `"tcp"`: TCP protocol (phổ biến nhất)
- `"udp"`: UDP protocol (cho game servers, DNS, etc.)

#### `target_host` (string, bắt buộc)
Hostname hoặc IP của server cần test:
- Domain name: `"mc.tahn.site"`, `"example.com"`
- IPv4: `"1.2.3.4"`
- IPv6: `"2001:db8::1"` (chỉ SOCKS5)

#### `target_port` (int, bắt buộc)
Port của server: `25565`, `80`, `443`, etc.

#### `send_hex` (string, optional)
Raw bytes cần gửi, định dạng HEX string:
- VD: `"1400860D6D632E7468616E2E696F2E766E63DD01"`
- Không có prefix `0x`
- Mỗi byte = 2 ký tự hex
- Để rỗng `""` nếu không cần gửi gì

**Cách chuyển đổi:**
```python
# Python
data = b'\x14\x00\x86\x06\rmc.tahn.io.vnc\xdd\x01'
hex_string = data.hex()  # "1400860D6D632E7468616E2E696F2E766E63DD01"
```

```bash
# Command line
echo -n "Hello" | xxd -p  # 48656c6c6f
```

#### `expect_response` (boolean, bắt buộc)
- `true`: Proxy chỉ hợp lệ nếu server reply
- `false`: Proxy hợp lệ nếu gửi packet thành công (không cần reply)

#### `response_min_size` (int, optional)
Số bytes tối thiểu trong response:
- `0` hoặc không set: Chấp nhận response bất kỳ kích thước
- `> 0`: Response phải >= số bytes này

VD: `10` = response phải >= 10 bytes

#### `timeout` (int, optional)
Timeout cho mỗi operation (giây):
- `0` hoặc không set: Dùng timeout mặc định từ `config.timeout`
- `> 0`: Custom timeout

## Ví dụ cấu hình

### 1. Minecraft Server (ví dụ của bạn)

```json
{
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
  "proxy-type": "socks5"
}
```

**Giải thích:**
- Kết nối TCP tới mc.tahn.site:25565
- Gửi Minecraft handshake packet
- Đợi server reply
- Reply phải >= 10 bytes
- Timeout 5 giây

### 2. Custom Game Server

```json
{
  "raw_socket_test": {
    "enabled": true,
    "protocol": "tcp",
    "target_host": "game.example.com",
    "target_port": 7777,
    "send_hex": "DEADBEEF",
    "expect_response": true,
    "response_min_size": 4,
    "timeout": 3
  },
  "proxy-type": "socks4"
}
```

### 3. Simple TCP Connection Test

```json
{
  "raw_socket_test": {
    "enabled": true,
    "protocol": "tcp",
    "target_host": "8.8.8.8",
    "target_port": 53,
    "send_hex": "",
    "expect_response": false,
    "response_min_size": 0,
    "timeout": 2
  },
  "proxy-type": "http"
}
```

**Giải thích:**
- Chỉ test kết nối TCP
- Không gửi data
- Không đợi response
- Chỉ cần connect thành công

### 4. UDP Protocol Example

```json
{
  "raw_socket_test": {
    "enabled": true,
    "protocol": "udp",
    "target_host": "dns.google",
    "target_port": 53,
    "send_hex": "00010100000100000000000003777777076578616D706C6503636F6D0000010001",
    "expect_response": true,
    "response_min_size": 20,
    "timeout": 3
  },
  "proxy-type": "socks5"
}
```

**Note:** UDP chỉ work với SOCKS5!

## Proxy Types

### HTTP Proxy
```json
{"proxy-type": "http"}
```

- Dùng HTTP CONNECT method
- Tạo tunnel tới target
- Gửi raw bytes qua tunnel

**Flow:**
```
Client → HTTP Proxy (CONNECT mc.tahn.site:25565) → Target Server
Client → Send raw bytes → Target Server
Target Server → Response → Client
```

### SOCKS4 Proxy
```json
{"proxy-type": "socks4"}
```

- Chỉ hỗ trợ IPv4
- Chỉ hỗ trợ TCP
- Tự động resolve hostname → IP

**Flow:**
```
Client → SOCKS4 Proxy (handshake) → Target Server
Client → Send raw bytes → Target Server
Target Server → Response → Client
```

### SOCKS5 Proxy
```json
{"proxy-type": "socks5"}
```

- Hỗ trợ IPv4, IPv6, domain name
- Hỗ trợ TCP và UDP
- Không cần resolve hostname

**Flow:**
```
Client → SOCKS5 Proxy (handshake) → Target Server
Client → Send raw bytes → Target Server
Target Server → Response → Client
```

## Status Codes

### Success (lưu vào output file)
- Proxy kết nối thành công
- Gửi packet thành công
- Nhận response (nếu `expect_response: true`)
- Response size đủ lớn (nếu có `response_min_size`)

### ProxyErr
- Không thể kết nối tới proxy
- Proxy handshake thất bại
- Không thể establish tunnel
- Lỗi network

### TimeoutErr
- Kết nối timeout
- Gửi packet timeout
- Đợi response timeout

### StatusCodeErr
- Response size < `response_min_size`
- Proxy từ chối connect request
- Server không reply (khi `expect_response: true`)

## Debugging

### Tạo hex string từ binary data

**Python:**
```python
import binascii

# Method 1: From bytes
data = b'\x14\x00\x86\x06\rmc.tahn.io.vnc\xdd\x01'
hex_str = data.hex()
print(hex_str)  # 1400860D6D632E7468616E2E696F2E766E63DD01

# Method 2: From string
text = "Hello"
hex_str = text.encode().hex()
print(hex_str)  # 48656c6c6f
```

**Node.js:**
```javascript
// From Buffer
const buf = Buffer.from([0x14, 0x00, 0x86, 0x06]);
const hex = buf.toString('hex');
console.log(hex);  // 14008606

// From string
const text = "Hello";
const hex2 = Buffer.from(text).toString('hex');
console.log(hex2);  // 48656c6c6f
```

**Command line:**
```bash
# xxd
echo -n "Hello" | xxd -p

# hexdump
echo -n "Hello" | hexdump -e '16/1 "%02x" "\n"'

# od
echo -n "Hello" | od -An -tx1 | tr -d ' \n'
```

### Kiểm tra response

Thêm logging để xem response:
```go
// Trong http.go, thêm vào hàm test
if config.RawSocketTest.ExpectResponse {
    responseBuffer := make([]byte, 4096)
    n, err := conn.Read(responseBuffer)
    
    // DEBUG: Print response
    log.Printf("Response hex: %s\n", hex.EncodeToString(responseBuffer[:n]))
    log.Printf("Response size: %d bytes\n", n)
}
```

## Best Practices

### 1. Chọn timeout phù hợp
- Game servers: 3-5 giây
- Web services: 2-3 giây
- Custom protocols: tùy thuộc vào độ trễ

### 2. Set response_min_size hợp lý
- Quá nhỏ (0-5): Có thể accept garbage response
- Quá lớn: Có thể reject valid response
- Recommended: Test thủ công để xác định size

### 3. Protocol selection
- HTTP proxy: Versatile, work với most protocols
- SOCKS4: Faster handshake, chỉ IPv4
- SOCKS5: Full featured, support UDP

### 4. Testing workflow
```bash
# 1. Test với 1 proxy đơn
echo "1.2.3.4:8080" | ./scanner -cfg config.minecraft.json

# 2. Test với file nhỏ
./scanner -in test_proxies.txt -cfg config.minecraft.json

# 3. Production với file lớn
./scanner -in all_proxies.txt -cfg config.minecraft.json -o valid.txt
```

## Troubleshooting

### Proxy không pass
1. Check proxy type đúng chưa (http/socks4/socks5)
2. Check target_host:port accessible không
3. Check send_hex format đúng không
4. Test thủ công với netcat/telnet

### Timeout errors
1. Tăng timeout value
2. Check network latency
3. Reduce thread count
4. Check target server có alive không

### Wrong response size
1. Test thủ công để xác định expected size
2. Set response_min_size = 0 để debug
3. Add logging để xem actual response

### Hex conversion issues
```python
# Validate hex string
def validate_hex(hex_str):
    try:
        bytes.fromhex(hex_str)
        return True
    except ValueError:
        return False

validate_hex("1400860D")  # True
validate_hex("14G0")      # False (G không phải hex)
```

## Kết hợp HTTP checking và Raw Socket

Bạn có thể kết hợp cả hai:

```json
{
  "check-sites": [
    {
      "url": "https://google.com",
      "status_code": 200,
      "response_contains": []
    }
  ],
  "raw_socket_test": {
    "enabled": true,
    "protocol": "tcp",
    "target_host": "mc.tahn.site",
    "target_port": 25565,
    "send_hex": "...",
    "expect_response": true
  }
}
```

**Note:** Khi `raw_socket_test.enabled = true`, nó sẽ **override** HTTP checking. Chỉ raw socket test được chạy.

Nếu muốn cả hai, set `enabled: false` cho raw socket test.
