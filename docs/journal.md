# Tech Stack cho Ginnungagap CLI

Dưới đây là đề xuất các thư viện (packages) cực kỳ chất lượng trong hệ sinh thái Go để xây dựng tool của bạn:

## 1. CLI Framework & Giao diện (TUI)
* **[Charmbracelet Bubble Tea](https://github.com/charmbracelet/bubbletea):** Nó dùng kiến trúc The Elm Architecture, giúp tạo ra các giao diện Terminal tương tác rất đẹp và mượt mà.
* **[Charmbracelet Lip Gloss](https://github.com/charmbracelet/lipgloss):** Đi kèm với Bubble Tea để style (tô màu, in đậm, tạo khung box) cho Terminal giống như viết CSS.
* **[Cobra](https://github.com/spf13/cobra):** Mặc dù Bubble Tea làm giao diện xuất sắc, bạn vẫn nên dùng Cobra để quản lý cấu trúc lệnh CLI (ví dụ: `ginnungagap scan`, `ginnungagap config`). Cobra là tiêu chuẩn vàng được dùng bởi Kubernetes, Docker, GitHub CLI.

## 2. Giao tiếp với Container Engine (Docker / Podman)
* **[Docker Engine SDK for Go](https://github.com/docker/docker/tree/master/client):** (`github.com/docker/docker/client`). Đây là thư viện chính chủ của Docker. Bạn có thể dùng nó để:
  * Pull images (ví dụ image node chứa honeypot).
  * Khởi tạo, start, stop, pause container.
  * Tương tác với Docker socket (`/var/run/docker.sock`) để ra lệnh commit container thành file `tar.gz` ngay khi có biến.
  * *(Lưu ý: Podman cung cấp Docker-compatible API, nên thư viện này vẫn dùng được với Podman socket ở hầu hết các case cơ bản).*

## 3. Quản lý File nén (Forensic Packing)
* **[archive/tar](https://pkg.go.dev/archive/tar) & [compress/gzip](https://pkg.go.dev/compress/gzip):** Các thư viện standard của Go (không cần cài thêm) để nén thư mục hoặc container filesystem thành `tar.gz` phục vụ cho việc ném lên Reddit hoặc gửi cho npm security team.

## 4. Giao tiếp với Falco (Monitoring & Triggers)
* Không nhất thiết phải dùng thư viện phức tạp. Khi Falco phát hiện rủi ro, nó có thể bắn Webhook qua **Falco Sidekick**. Bạn chỉ cần dùng standard `net/http` dựng một server để lắng nghe webhook từ Falco, từ đó trigger lệnh pause Docker thông qua Docker SDK.

## 5. Tiện ích khác
* **[Viper](https://github.com/spf13/viper):** Để đọc file cấu hình (`.yaml`, `.json`) hoặc lấy biến môi trường cho tool (ví dụ: cấu hình đường dẫn đến Docker socket, timeout, ...). Thường đi cặp với Cobra.
* **[Go-Spinner](https://github.com/briandowns/spinner) hoặc dùng [Bubbles (Spinner)](https://github.com/charmbracelet/bubbles):** Tạo hiệu ứng loading cực ngầu trong lúc chờ `npm install` chạy ngầm.

