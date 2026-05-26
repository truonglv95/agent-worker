# Agy Runner Integration Guide

Tài liệu này cung cấp hướng dẫn chi tiết để tích hợp các hệ thống bên ngoài (Frontend, Mobile App, Backend khác) với **Agy Runner** thông qua REST API.

## 1. Xác thực (Authentication)

Agy Runner sử dụng **JWT (JSON Web Token)** để bảo vệ các API thực thi lệnh. Bạn cần đăng nhập bằng tài khoản Admin để lấy token trước khi sử dụng các API khác.

### 1.1 Đăng nhập lấy Token
- **Endpoint**: `POST /api/v1/auth/login`
- **Content-Type**: `application/json`
- **Payload**:
  ```json
  {
    "username": "admin",
    "password": "your_admin_password"
  }
  ```
- **Response**:
  ```json
  {
    "access_token": "eyJhbGciOiJIUz...",
    "refresh_token": "d7a8s9d7a9s8d7..."
  }
  ```
> **Lưu ý**: Tất cả các API yêu cầu xác thực bên dưới đều phải được gửi kèm Header:
> `Authorization: Bearer <access_token>`

### 1.2 Làm mới Token (Refresh)
- **Endpoint**: `POST /api/v1/auth/refresh`
- **Payload**:
  ```json
  {
    "refresh_token": "d7a8s9d7a9s8d7..."
  }
  ```
- **Response**: Trả về `access_token` mới.

---

## 2. API Thực thi AI (Chat / Code)

Dùng để gọi AI thực hiện các tác vụ sinh chữ, lập trình, giải toán, v.v.

- **Endpoint**: `POST /api/v1/run`
- **Content-Type**: `application/json`
- **Payload**:
  ```json
  {
    "prompt": "Viết cho tôi một hàm tính tổng bằng Golang",
    "conversation_id": "workflow_123", 
    "stream": true 
  }
  ```
  *(Truyền `conversation_id` nếu bạn muốn AI nhớ ngữ cảnh của các lần chat trước).*

- **Response (Nếu `stream: false`)**: Trả về một khối JSON chứa toàn bộ kết quả.
  ```json
  {
    "output": "func sum(a int, b int) int { return a + b }"
  }
  ```

- **Response (Nếu `stream: true`)**: Trả về kết nối **Server-Sent Events (SSE)**. Nội dung trả về liên tục theo luồng:
  ```text
  data: {"text": "func sum("}
  data: {"text": "a int, b int) int {\n"}
  ...
  event: done
  data: {}
  ```

---

## 3. API Xử lý Ảnh (Image Editing / Generation)

Agy Runner có luồng xử lý ảnh chuyên dụng, tối ưu hóa việc người dùng tải ảnh lên và ra lệnh cho AI chỉnh sửa.

### 3.1 Upload ảnh lên Runner
- **Endpoint**: `POST /api/v1/upload`
- **Content-Type**: `multipart/form-data`
- **Payload**:
  - `file`: (Kiểu File) - Bức ảnh bạn muốn tải lên.
- **Response**:
  ```json
  {
    "message": "Upload successful",
    "path": "/tmp/runner_uploads/user_1/image.png"
  }
  ```

### 3.2 Tải ảnh từ Runner về Client
- **Endpoint**: `GET /api/v1/download?path=<absolute_path>`
- **Mô tả**: API Public dùng để hiển thị trực tiếp ảnh.
- **Ví dụ Frontend**:
  ```html
  <img src="https://<tunnel_url>/api/v1/download?path=/tmp/runner_uploads/user_1/image.png" />
  ```

### 3.3 Chỉnh sửa ảnh (All-in-one Endpoint)
Đây là Endpoint thông minh, hỗ trợ cả việc truyền `image_path` có sẵn HOẶC tải file trực tiếp lên và edit ngay lập tức.

- **Endpoint**: `POST /api/v1/edit-image`
- **Content-Type**: `multipart/form-data`
- **Payload**:
  - `prompt` (Text): "Xóa phông nền bức ảnh này"
  - `stream` (Text): "true" hoặc "false"
  - **Cách A (Upload trực tiếp)**: Đính kèm file vào field `image` (Kiểu File).
  - **Cách B (Dùng ảnh đã upload)**: Truyền field `image_path` (Text) bằng đường dẫn trả về từ API Upload.

- **Luồng xử lý**:
  - Runner sẽ ép AI thực hiện việc edit ảnh.
  - Tùy thuộc vào cấu hình môi trường của Runner:
    - Nếu có cấu hình Cloudflare R2: AI sẽ tự động đẩy ảnh lên R2 và trả về **Public Link R2**.
    - Nếu không có R2: AI sẽ trả về đường dẫn được cấu trúc sẵn để Frontend dễ dàng dùng API Download: `/api/v1/download?path=/tmp/runner_uploads/.../edited.png`.

---

## 4. Tích hợp Streaming vào Frontend (Gợi ý mã nguồn JS)

Nếu bạn dùng `stream: true`, đây là cách để Frontend JavaScript (React/Vue/Vanilla) đọc dữ liệu Real-time như ChatGPT:

```javascript
import { fetchEventSource } from '@microsoft/fetch-event-source';

async function runAI(prompt) {
  let fullResponse = "";
  
  await fetchEventSource('https://<runner_url>/api/v1/run', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer YOUR_ACCESS_TOKEN'
    },
    body: JSON.stringify({
      prompt: prompt,
      stream: true
    }),
    onmessage(ev) {
      if (ev.event === 'done') {
        console.log("Finished!");
        return;
      }
      // Parse data
      const data = JSON.parse(ev.data);
      if (data.text) {
        fullResponse += data.text;
        console.log("Chunk received:", data.text);
      } else if (data.error) {
        console.error("AI Error:", data.error, data.stderr);
      }
    }
  });
}
```
