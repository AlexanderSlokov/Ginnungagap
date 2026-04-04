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
* **[archive/tar](https://pkg.go.dev/archive/tar) and [compress/gzip](https://pkg.go.dev/compress/gzip):** Các thư viện standard của Go (không cần cài thêm) để nén thư mục hoặc container filesystem thành `tar.gz` phục vụ cho việc ném lên Reddit hoặc gửi cho npm security team.

## 4. Giao tiếp với Falco (Monitoring & Triggers)
* Không nhất thiết phải dùng thư viện phức tạp. Khi Falco phát hiện rủi ro, nó có thể bắn Webhook qua **Falco Sidekick**. Bạn chỉ cần dùng standard `net/http` dựng một server để lắng nghe webhook từ Falco, từ đó trigger lệnh pause Docker thông qua Docker SDK.

## 5. Tiện ích khác
* **[Viper](https://github.com/spf13/viper):** Để đọc file cấu hình (`.yaml`, `.json`) hoặc lấy biến môi trường cho tool (ví dụ: cấu hình đường dẫn đến Docker socket, timeout, ...). Thường đi cặp với Cobra.
* **[Go-Spinner](https://github.com/briandowns/spinner) hoặc dùng [Bubbles (Spinner)](https://github.com/charmbracelet/bubbles):** Tạo hiệu ứng loading cực ngầu trong lúc chờ `npm install` chạy ngầm.

# Design Pattern

## 1. Kiến trúc tổng thể: Clean Architecture (chú trọng Ports & Adapters)

Dự án được chia thành các lớp (layers):

   • Core/Domain: Chứa các models (ví dụ: Container, Alert, ForensicReport).
   • Ports (Interfaces): Định nghĩa "hợp đồng" giao tiếp. Ví dụ: type ContainerEngine interface { Pause(id string), Commit(id string) }.
   • Adapters (Infrastructure): Nơi thực thi thực tế. Lớp này sẽ chứa code gọi Docker SDK (docker_adapter.go) hoặc code đọc file log của Falco (falco_watcher.go).
   • Application/Service: Nơi chứa business logic. Nó sẽ parse log của Falco, rồi gọi hàm Pause của Docker Adapter.
   
Lợi ích: UI (CLI/TUI) chỉ gọi đến Application Layer. Tương lai bạn muốn đổi Docker sang Podman, bạn chỉ cần viết thêm một PodmanAdapter mà không phải đập đi viết lại phần UI hay Core.

## 2. Design Pattern cho CLI: Command Pattern (Cobra)
   
Thư viện Cobra bản thân nó đã áp dụng triệt để Command Pattern.

   • Mỗi lệnh CLI (fourg scan, fourg config, fourg daemon) sẽ là một struct/đối tượng Command riêng biệt.
   • Pattern này giúp dễ dàng bóc tách cờ (flags), tham số (args) và logic khởi chạy của từng lệnh mà không bị rối rắm.

## 3. Design Pattern cho TUI: The Elm Architecture (MVU – Model, View, Update)

Thư viện Bubble Tea bắt buộc phải theo kiến trúc MVU (Model-View-Update). Đây là một biến thể rất mạnh mẽ của State Machine dành cho UI:
   • Model: Một struct chứa TOÀN BỘ trạng thái của giao diện (ví dụ: isScanning bool, currentLog string, alerts []Alert).
   • Update: Một hàm duy nhất nhận vào các sự kiện (Events/Messages - ví dụ: user nhấn phím q, hoặc có message báo ScanComplete). Hàm này xử lý event và trả về một Model mới.
   • View: Một hàm thuần túy (pure function) nhận Model và render ra chuỗi String (giao diện Terminal) dựa trên trạng thái hiện tại.

Lớp Update của Bubble Tea sẽ giao tiếp với Application Layer (chạy ngầm trong các Goroutines) thông qua việc gửi/nhận tea.Cmd (Messages).

## 4. Cấu trúc thư mục

```text
Ginnungagap/
├── cmd/
│   └── fourg/           # Entrypoint chính của CLI
│       ├── root.go      # Lệnh gốc
│       ├── scan.go      # Lệnh `fourg scan`
│       └── daemon.go    # Lệnh chạy ngầm nghe Falco
├── internal/            # Code private, không cho project khác import
│   ├── ui/              # Chứa các component Bubble Tea (Model, View, Update)
│   ├── engine/          # Application Layer (Ginnungagap workflow logic)
│   ├── docker/          # Adapter giao tiếp với Docker SDK
│   └── falco/           # Adapter đọc/parse log Falco
├── pkg/                 # Code có thể tái sử dụng được nếu cần
├── docs/                
├── docker-compose.yml
├── Dockerfile
└── main.go              # Chỉ đơn giản là gọi cmd.Execute()
```